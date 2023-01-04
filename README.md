# Snapdrop 
*Original project*
[Snapdrop](https://github.com/RobinLinus/snapdrop): local file sharing in your browser. Inspired by Apple's Airdrop.
# go-snapdrop
Features:  
1. Rewrite the backend using go.
2. Increase the identification of private network. eg: 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16
3. Single file execution

Usage:
```shell
 -a string
        http service address (default "0.0.0.0:8080")
  -c string
        SSL Certificate File
  -h    Print the help text
  -k string
        SSL Key File
```
Access the local LAN network IP: 8080 to transfer files to each other. eg: 192.168.1.104:8080
