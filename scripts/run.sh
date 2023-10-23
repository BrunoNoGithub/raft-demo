#!/bin/bash
# run.sh will automatically run with varying number of clients
# server and replicas, and possible logs, must be setup before running

# Possibly find a way to autoamtically detect path
# Same may go for genClients.sh
path=/home/bruno/Documents/LAPESD/raft-demo/
local=.

#clientsFolders=(1 4 7 10 13 16 19)

# for kvstore
#numClients=(1 4 7 10 13 16 19)

# for diskstore
numClients=(1 7 13 19 25 31 37)

#numClients=(1 12 23 34 45 56)
#numClients=(1 6 11 16 21 26 31)
#numClients=(1 3 5 7 9 11 13)

dataSizeOptions=(1) #0: 128B, 1: 1KB, 2: 4KB
execTime=60 #seconds
numDiffHash=1000000

#Cmd line parameters:
# 1 - experimentFolderName
# 2 - Set to 1 to logLatency (same folder as where file is run, so Client)
# 3- configFilename

#Checks if number of arguments is not 2 or 3 ($# is number of arguments passed, and -ne is not equal)
if [[ $# -ne 2 ]] && [[ $# -ne 3 ]]
then
	echo "usage 2 args: $0 'experimentFolderName' 'logLatency(0: false; 1: true)'"
	echo "usage 3 args: $0 'experimentFolderName' 'logLatency(0: false; 1: true)' 'configFilename'"
	exit 1
fi

echo "started...."
for j in ${dataSizeOptions[*]}
do
	#for i in ${numClients[*]}
	for (( i=0; i<${#numClients[@]}; ++i ));
    do

        if [ ${numClients[i]} -eq 0 ]; then

            # used to distribute client load generation on diff nodes
            sleep ${execTime}

        # runs local script
        # previously assigned numclients, execTime and number of hashes, as well as dataSizeOption
        # loglatency(true/false) and configFilename passed as parameters
        else
            if [[ $# -eq 2 ]]; then
            	$local/genClients.sh 1 ${numClients[i]} ${execTime} ${numDiffHash} ${j} ${2}
            else
            	$local/genClients.sh 1 ${numClients[i]} ${execTime} ${numDiffHash} ${j} ${2} ${3}
            fi

            # Disable for kube monitoring...
            # if [ $2 -eq "1" ]; then
            # 	mv $path/client/*.out ${local}/${1}/${j}/${clientsFolders[i]}/${clientsFolders[i]}c-latency.out
            # fi
        fi
        echo "Finished running experiment for ${numClients[i]} clients."; echo ""

        # waiting for server reasource dealloc...
        sleep 10s
    done

    # Disable for kube monitoring...
    # if [ $2 -eq "1" ]; then
    # 	mv $path/$app/*.out ${local}/${1}/${j}/
    # fi

    echo "Finished clients for $j data size."; echo ""
done

echo "Finished!"; echo ""
