package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type Guest struct {
	conn net.Conn
	host *Host
	logger *log.Logger
	incoming chan string
	outgoing chan string
}

func handleGuest(h *Host, conn net.Conn) *Guest {
	g := new(Guest)
	g.conn = conn
	g.host = h
	g.logger = log.New(os.Stdout, fmt.Sprint("[Client ", conn.RemoteAddr(), "] "), log.LstdFlags)
	g.incoming = make(chan string)
	g.outgoing = make(chan string)
	go g.Reader()
	go g.Writer()
	return g
}

// reads the socket and writes to the incoming channel
func (g *Guest) Reader() {
	// need to recover if the incoming channel is closed
	defer func() {
		if r := recover(); r != nil {
			g.logger.Println("incoming channel was closed")
		}
	}()

	r := bufio.NewReader(g.conn)
	for {
		g.conn.SetReadDeadline(time.Now().Add(30*time.Second))
		resp, err := r.ReadString('\n')
		if err != nil {
			g.logger.Println(err)
			close(g.incoming)
			g.logger.Println("closing Reader")
			return
		}
		g.incoming <- resp
	}
}

// reads the outgoing channel and writes to the socket
func (g *Guest) Writer() {
	w := bufio.NewWriter(g.conn)
	for message := range g.outgoing {
		g.conn.SetWriteDeadline(time.Now().Add(30*time.Second))
		w.WriteString(message)
		err := w.Flush()
		if err != nil {
			g.logger.Println(err)
			close(g.outgoing)
			g.logger.Println("closing Writer")
			return
		}
	}
	g.logger.Println("closing Writer")
}

func (g *Guest) Close() {
	close(g.incoming)
	close(g.outgoing)
	g.conn.Close()
}
