package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
)

var addr = flag.String("addr", "0.0.0.0:8080", "http service address")

//go:embed client
var local embed.FS

func main() {
	flag.Parse()
	server := SnapdropServer{}
	server.Init()
	root, _ := fs.Sub(local, "client")
	http.Handle("/", http.FileServer(http.FS(root)))
	http.HandleFunc("/server/", server.OnHeaders)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
