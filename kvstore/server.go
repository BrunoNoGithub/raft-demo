package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Server stores the state between every client
type Server struct {
	clients  []*Session // # List of Session(s), per client
	joins    chan net.Conn // # Channel for new connections
	incoming chan *Request // # Channel

	t          *time.Timer
	req        uint64 // #Number of requests?
	throughput *os.File
	kvstore    *Store // # KVS for server, each server contains a single KVS
}

// NewServer constructs and starts a new Server
func NewServer(ctx context.Context, s *Store) *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan *Request),
		req:      0,
		kvstore:  s,
		t:        time.NewTimer(time.Second),
	}
	svr.throughput = createFile(svrID + "-throughput.out")

	// # Two concurrent go func for listening and monitoring, using the passed ctx context
	go svr.Listen(ctx)
	go svr.monitor(ctx)
	return svr
}

// Exit closes the raft context and releases any resources allocated
func (svr *Server) Exit() {

	svr.kvstore.raft.Shutdown()
	if svr.kvstore.Logging {
		svr.kvstore.LogFile.Close()
	}
	// #Disconnect from every client
	for _, v := range svr.clients {
		v.Disconnect()
	}
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}
}

// SendUDP sends a UDP reply to a client listening on 'addr'
func (svr *Server) SendUDP(addr string, message string) {
	conn, _ := net.Dial("udp", addr)
	defer conn.Close()
	conn.Write([]byte(message))
}

// HandleRequest handles the client requistion, checking if it matches the right syntax
// before proposing it to the FSM
// # Where is the FSM called?
func (svr *Server) HandleRequest(cmd *Request) {

	data := bytes.TrimSuffix(cmd.Command, []byte("\n"))
	fmt.Println("data: ",data)
	if err := svr.kvstore.Propose(data, svr, cmd.Ip); err != nil {
		svr.kvstore.logger.Error(fmt.Sprintf("Failed to propose message: %q, error: %s\n", data, err.Error()))
	}
	atomic.AddUint64(&svr.req, 1)
}

// Join threats a join requisition from clients to the Server state
func (svr *Server) Join(ctx context.Context, connection net.Conn) {
	client := NewSession(connection) // # Get client(session) from connection channel
	svr.clients = append(svr.clients, client) // # Put client in server client list

	go func() { // # Concurrently look for data from client
		for {
			select {
			case <-ctx.Done():
				return

			case data := <-client.incoming: // # Get incoming request from client incoming chan
				svr.incoming <- data
			}
		}
	}()
}

// Listen receives incoming messagens and new connections from clients
func (svr *Server) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case data := <-svr.incoming:
			svr.HandleRequest(data)

		case conn := <-svr.joins:
			svr.Join(ctx, conn)
		}
	}
}

func (svr *Server) monitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-svr.t.C:
			cont := atomic.SwapUint64(&svr.req, 0)
			//svr.kvstore.logger.Info(fmt.Sprintf("Thoughput(cmds/s): %d", cont))
			svr.throughput.WriteString(fmt.Sprintf("%v\n", cont))
			svr.t.Reset(time.Second)
		}
	}
}

// Legacy code, used only on ad-hoc message formats.
// # No longer used, seemingly
func validateReq(requisition string) bool {

	requisition = strings.ToLower(requisition)
	splited := strings.Split(requisition, "-")

	if splited[1] == "set" {
		return len(splited) >= 3
	} else if splited[1] == "get" || splited[1] == "delete" {
		return len(splited) >= 2
	} else {
		return false
	}
}
