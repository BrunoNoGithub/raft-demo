package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"
)

const (
	// Used in catastrophic fault models, where crash faults must be recoverable even if
	// all nodes presented in the consensus cluster are down. Always set to false in any
	// other cases, because this strong assumption greatly degradates performance.
	catastrophicFaults = false

	// Each second writes current throughput to stdout.
	monitoringThroughtput = false
)

// Custom configuration over default for testing
func configRaft() *raft.Config {

	config := raft.DefaultConfig()
	config.SnapshotInterval = 24 * time.Hour
	config.SnapshotThreshold = 2 << 62
	config.LogLevel = "WARN"
	return config
}

// Logger struct represents the Logger process state. Member of the Raft cluster as a
// non-Voter participant and thus, just recording proposed commands to the FSM
type Logger struct {
	log     *log.Logger
	raft    *raft.Raft
	LogFile *os.File

	monit bool
	req   uint64
}

// NewLogger constructs a new Logger struct and its dependencies
func NewLogger(uniqueID string) *Logger {

	l := &Logger{
		log: log.New(os.Stderr, "[chatLogger] ", log.LstdFlags),
		req: 0,
	}

	if recovHandlerAddr != "" {
		l.ListenStateTransfer(recovHandlerAddr)
	}

	var flags int
	logFileName := *logfolder + "log-file-" + uniqueID + ".txt"
	if catastrophicFaults {
		flags = os.O_SYNC | os.O_WRONLY
	} else {
		flags = os.O_WRONLY
	}

	if _, exists := os.Stat(logFileName); exists == nil {
		l.LogFile, _ = os.OpenFile(logFileName, flags, 0644)
	} else if os.IsNotExist(exists) {
		l.LogFile, _ = os.OpenFile(logFileName, os.O_CREATE|flags, 0644)
	} else {
		log.Fatalln("Could not create log file:", exists.Error())
	}

	if monitoringThroughtput {
		l.monit = true
		l.monitor()
	}
	return l
}

// StartRaft initializes the node to be part of the raft cluster, the Logger process procedure
// is differente because its will never the first initialize node and never a candidate to leadership
func (lgr *Logger) StartRaft(localID, raftAddr string) error {

	// Setup Raft configuration.
	config := configRaft()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Using just in-memory storage (could use boltDB in the key-value application)
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()

	// Create a fake snapshot store
	dir := "checkpoints/" + localID
	snapshots, err := raft.NewFileSnapshotStore(dir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(lgr), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	lgr.raft = ra
	return nil
}

func (lgr *Logger) monitor() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			cont := atomic.SwapUint64(&lgr.req, 0)
			fmt.Println(cont)
		}
	}()
}

// UnsafeStateRecover ...
func (lgr *Logger) UnsafeStateRecover(logIndex uint64, activePipe net.Conn) error {

	// Create a read-only file descriptor
	logFileName := *logfolder + "log-file-" + logID + ".txt"
	fd, _ := os.OpenFile(logFileName, os.O_RDONLY, 0644)
	defer fd.Close()

	logFileContent, err := readAll(fd)
	if err != nil {
		return err
	}

	signalError := make(chan error, 0)
	go func(dataToSend []byte, pipe net.Conn, signal chan<- error) {

		_, err := pipe.Write(dataToSend)
		signal <- err

	}(logFileContent, activePipe, signalError)
	return <-signalError
}

// ListenStateTransfer ...
func (lgr *Logger) ListenStateTransfer(addr string) {

	go func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to bind connection at %s: %s", addr, err.Error())
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			request, _ := bufio.NewReader(conn).ReadString('\n')

			data := strings.Split(request, "-")
			if len(data) != 2 {
				log.Fatalf("incorrect state request, got: %s", data)
			}

			data[1] = strings.TrimSuffix(data[1], "\n")
			requestedLogIndex, _ := strconv.Atoi(data[1])

			err = lgr.UnsafeStateRecover(uint64(requestedLogIndex), conn)
			if err != nil {
				log.Fatalf("failed to transfer log to node located at %s: %s", data[0], err.Error())
			}

			err = conn.Close()
			if err != nil {
				log.Fatalf("Error encountered on connection close: %s", err.Error())
			}
		}
	}()
}

// readAll is a slightly derivation of 'ioutil.ReadFile()'. It skips the file descriptor creation
// and is declared to avoid unecessary dependency from the whole ioutil package.
// 'A little copying is better than a little dependency.'
func readAll(fileDescriptor *os.File) ([]byte, error) {
	// It's a good but not certain bet that FileInfo will tell us exactly how much to
	// read, so let's try it but be prepared for the answer to be wrong.
	var n int64 = bytes.MinRead

	if fi, err := fileDescriptor.Stat(); err == nil {
		// As initial capacity for readAll, use Size + a little extra in case Size
		// is zero, and to avoid another allocation after Read has filled the
		// buffer. The readAll call will read into its allocated internal buffer
		// cheaply. If the size was wrong, we'll either waste some space off the end
		// or reallocate as needed, but in the overwhelmingly common case we'll get
		// it just right.
		if size := fi.Size() + bytes.MinRead; size > n {
			n = size
		}
	}
	return func(r io.Reader, capacity int64) (b []byte, err error) {
		// readAll reads from r until an error or EOF and returns the data it read
		// from the internal buffer allocated with a specified capacity.
		var buf bytes.Buffer
		// If the buffer overflows, we will get bytes.ErrTooLarge.
		// Return that as an error. Any other panic remains.
		defer func() {
			e := recover()
			if e == nil {
				return
			}
			if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
				err = panicErr
			} else {
				panic(e)
			}
		}()
		if int64(int(capacity)) == capacity {
			buf.Grow(int(capacity))
		}
		_, err = buf.ReadFrom(r)
		return buf.Bytes(), err
	}(fileDescriptor, n)
}

var logID string
var raftAddr string
var joinAddr string
var recovHandlerAddr string

var logfolder *string

func init() {
	flag.StringVar(&logID, "id", "", "Set the logger unique ID")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinAddr, "join", ":13000", "Set join address to an already configured raft node")
	flag.StringVar(&recovHandlerAddr, "hrecov", "", "Set port id to receive state transfer requests from the application log")

	logfolder = flag.String("logfolder", "", "log received commands to a file at specified destination folder using Journey")
}

func main() {

	flag.Parse()
	if logID == "" {
		log.Fatalln("must set a logger ID, run with: ./logger -id 'logID'")
	}

	listOfLogIds := strings.Split(logID, ",")
	numDiffIds := countDiffStrInSlice(listOfLogIds)

	listOfRaftAddrs := strings.Split(raftAddr, ",")
	numDiffRaft := countDiffStrInSlice(listOfRaftAddrs)

	listOfJoinAddrs := strings.Split(joinAddr, ",")
	numDiffServices := countDiffStrInSlice(listOfJoinAddrs)

	if numDiffServices != numDiffIds || numDiffIds != numDiffRaft || numDiffRaft != numDiffServices {
		log.Fatalln("must run with the same number of unique IDs, raft and join addrs: ./logger -id 'X,Y' -raft 'A,B' -join 'W,Z'")
	}

	loggerInstances := make([]*Logger, numDiffServices)
	for i := 0; i < numDiffServices; i++ {
		go func(j int) {

			loggerInstances[j] = NewLogger(listOfLogIds[j])
			if err := loggerInstances[j].StartRaft(listOfLogIds[j], listOfRaftAddrs[j]); err != nil {
				log.Fatalf("failed to start raft cluster: %s", err.Error())
			}
			if err := sendJoinRequest(listOfLogIds[j], listOfRaftAddrs[j], listOfJoinAddrs[j]); err != nil {
				log.Fatalf("failed to send join request to node at %s: %s", listOfJoinAddrs[j], err.Error())
			}
		}(i)
	}

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	for _, l := range loggerInstances {
		l.LogFile.Close()
	}
}

func sendJoinRequest(logID, raftAddr, joinAddr string) error {
	joinConn, err := net.Dial("tcp", joinAddr)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(joinConn, logID+"-"+raftAddr+"-"+"false"+"\n")
	if err != nil {
		return err
	}

	err = joinConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func countDiffStrInSlice(elements []string) int {

	foundMarker := make(map[string]bool, len(elements))
	numDiff := 0

	for _, str := range elements {
		if !foundMarker[str] {
			foundMarker[str] = true
			numDiff++
		}
	}
	return numDiff
}
