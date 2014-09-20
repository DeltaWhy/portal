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
	guests chan *Guest
	closed bool
}

func handleHost(conn net.Conn) *Host {
	h := new(Host)
	h.conn = conn
	h.logger = log.New(os.Stdout, fmt.Sprint("[", conn.RemoteAddr(), "] "), log.LstdFlags)
	h.state = Unauthed
	h.incoming = make(chan string)
	h.outgoing = make(chan string)
	h.guests = make(chan *Guest)
	h.closed = false
	go h.Reader()
	go h.Writer()
	go hostSetup(h)
	time.Sleep(15*time.Second)
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
			h.Close()
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
			h.Close()
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
			if err.Error() == "use of closed network connection" {
				break
			} else {
				log.Println(err)
				continue
			}
		}
		h.logger.Println(conn.RemoteAddr(), " connected")
		h.guests <- handleGuest(h, conn)
	}
	h.logger.Println("Listener closed cleanly")
}

func (h *Host) Close() {
	if !h.closed {
		h.closed = true
		close(h.incoming)
		close(h.outgoing)
		if h.outside != nil {
			h.outside.Close()
		}
		h.conn.Close()
		close(h.guests)
	}
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

	h.logger.Println("auth response:", resp)

	h.outgoing <- "auth OK\n"

	resp, ok = <-h.incoming
	if !ok {
		h.logger.Println("closed before gameMeta")
		return
	}
	h.logger.Println("gameMeta:", resp)

	var err error
	h.outside, err = net.Listen("tcp", ":")
	if err != nil {
		h.logger.Println(err)
		h.outgoing <- "error opening outside port"
		h.Close()
		return
	}
	h.outgoing <- fmt.Sprint("opened outside ", h.outside.Addr(), "\n")
	go h.Listener()
	h.state = Ready

	gs := make(map[uint32]*Guest)
	handlerLoop:
	for {
		select {
		case message, ok := <-h.incoming:
			if ok {
				for _, g := range gs {
					g.outgoing <- message
				}
			} else {
				break handlerLoop
			}
		case g, ok := <-h.guests:
			if ok {
				h.logger.Println("handler got guest")
				gs[g.id] = g
				h.outgoing <- fmt.Sprint("got guest ", g.id, "\n")
			} else {
				break handlerLoop
			}
		}
	}
	for _, g := range gs {
		g.Close()
	}
}
