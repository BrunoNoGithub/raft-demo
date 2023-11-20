
#!/kvstore
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/kvstore --command "go build" &
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/kvstore --command "./kvstore -id node0 -hjoin :13000" &
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/kvstore --command "./kvstore -id node1 -port :11001 -raft :12001 -join :13000" &
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/kvstore --command "./kvstore -id node2 -port :11002 -raft :12002 -join :13000" &

#!/logger
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/logger --command "go build" &
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/logger --command  "./logger -id log1 -raft :12003 -join :13000" &
gnome-terminal --working-directory=/home/bruno/Documents/LAPESD/raft-demo/logger --command  "./logger -id log2 -raft :12004 -join :13000" &

#!/client
go build &

#!
./run.sh ~/tests 1

#chmod +x file.bsk
