package main

import (
	"log"
	"math/rand"
	"net"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	log.SetPrefix("[portald] ")
	srv, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on ", srv.Addr())
	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(conn.RemoteAddr(), " connected")
		handleHost(conn)
	}
}
