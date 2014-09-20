package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type HostState int

const (
	Unauthed HostState = iota
	Authing
	Authed
	Ready
)

type Host struct {
	conn net.Conn
	logger *log.Logger
	outside net.Listener
	state HostState
	incoming chan string
	outgoing chan string
}

func handleHost(conn net.Conn) *Host {
	h := new(Host)
	h.conn = conn
	h.logger = log.New(os.Stdout, fmt.Sprint("[", conn.RemoteAddr(), "] "), log.LstdFlags)
	h.state = Unauthed
	h.incoming = make(chan string)
	h.outgoing = make(chan string)
	go h.Reader()
	go h.Writer()
	go hostSetup(h)
	time.Sleep(5*time.Second)
	h.Close()
	return h
}

// reads the socket and writes to the incoming channel
func (h *Host) Reader() {
	// need to recover if the incoming channel is closed
	defer func() {
		if r := recover(); r != nil {
			h.logger.Println("incoming channel was closed")
		}
	}()

	r := bufio.NewReader(h.conn)
	for {
		h.conn.SetReadDeadline(time.Now().Add(30*time.Second))
		resp, err := r.ReadString('\n')
		if err != nil {
			h.logger.Println(err)
			close(h.incoming)
			h.logger.Println("closing Reader")
			return
		}
		h.incoming <- resp
	}
}

// reads the outgoing channel and writes to the socket
func (h *Host) Writer() {
	w := bufio.NewWriter(h.conn)
	for message := range h.outgoing {
		h.conn.SetWriteDeadline(time.Now().Add(30*time.Second))
		w.WriteString(message)
		err := w.Flush()
		if err != nil {
			h.logger.Println(err)
			close(h.outgoing)
			h.logger.Println("closing Writer")
			return
		}
	}
	h.logger.Println("closing Writer")
}

// listens for incoming guest connections
func (h *Host) Listener() {
	h.logger.Println("listening on ", h.outside.Addr())
	for {
		conn, err := h.outside.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		h.logger.Println(conn.RemoteAddr(), " connected")
		handleGuest(h, conn)
	}
}

func (h *Host) Close() {
	close(h.incoming)
	close(h.outgoing)
	h.conn.Close()
}

func hostSetup(h *Host) {
	h.outgoing <- "auth packet\n"
	h.state = Authing

	resp, ok := <-h.incoming
	if !ok {
		h.logger.Println("closed before authing")
		return
	}
	h.state = Authed

	h.logger.Println(resp)
	h.logger.Println("authed")
}
