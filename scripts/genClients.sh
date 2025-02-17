#!/bin/bash
#path should be handled in a better way...
goSource=/home/bruno/Documents/raft-demo/client
#will depend on wether run.sh was called with 2 or 3 parameters
if [[ $# -ne 6 ]] && [[ $# -ne 7 ]]
then
	echo "usage with 6 args: $0 'mode(0: numMessages; 1: execTime)' 'numberClients' '(reqForeachClient||execTime)' 'diffHashKeys' 'dataChoice(0: 128B, 1: 1KB, 2: 4KB)' 'logLatency(0: false; 1: true)'"; echo ""
	echo "usage with 7 args: $0 'mode(0: numMessages; 1: execTime)' 'numberClients' '(reqForeachClient||execTime)' 'diffHashKeys' 'dataChoice(0: 128B, 1: 1KB, 2: 4KB)' 'logLatency(0: false; 1: true)' 'configFilename'"
	exit 1
fi

go clean -testcache
#test choice hardcoded to 1 in run.sh
#echo $1,$2,$3,$4,$5,$6;
if [ $1 -eq "0" ]; then

	if [[ $# -eq 6 ]]; then
		go test $goSource -run TestNumMessagesKvstore -v -count 1 -clients=${2} -req=${3} -key=${4} -data=${5} -log=${6}
	else
		go test $goSource -run TestNumMessagesKvstore -v -count 1 -clients=${2} -req=${3} -key=${4} -data=${5} -log=${6} -config=${7}
	fi

elif [ $1 -eq "1" ]; then

	if [[ $# -eq 6 ]]; then
		go test $goSource -run TestClientTimeKvstore -v -count 1 -clients=${2} -time=${3} -key=${4} -data=${5} -log=${6}
	else
		go test $goSource -run TestClientTimeKvstore -v -count 1 -clients=${2} -time=${3} -key=${4} -data=${5} -log=${6} -config=${7}
		#0/1- number of clients active/lifetime of client//number of unique keys/dataSize (0 to 2)/Logging bool(mustlog)
	fi
fi
