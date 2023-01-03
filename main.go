package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
)

//go:embed client
var local embed.FS

func main() {
	addr := flag.String("a", "0.0.0.0:8080", "http service address")
	cert := flag.String("c", "", "SSL Certificate File")
	key := flag.String("k", "", "SSL Key File")
	help := flag.Bool("h", false, "Print the help text")
	flag.Parse()
	if *help {
		flag.Usage()
		return
	}
	server := SnapdropServer{}
	server.Init()
	root, _ := fs.Sub(local, "client")
	http.Handle("/", http.FileServer(http.FS(root)))
	http.HandleFunc("/server/", server.OnHeaders)
	if *cert == "" || *key == "" {
		log.Printf("Snapdrop Listening on %s", *addr)
		err := http.ListenAndServe(*addr, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		log.Printf("Snapdrop Listening on %s With TLS", *addr)
		err := http.ListenAndServeTLS(*addr, *cert, *key, nil)
		if err != nil {
			log.Fatal("ListenAndServeTLS: ", err)
		}
	}
}
