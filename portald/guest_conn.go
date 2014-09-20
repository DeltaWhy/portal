package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"github.com/DeltaWhy/portal/libportal"
)

type Guest struct {
	conn net.Conn
	host *Host
	logger *log.Logger
	id uint32
	closed bool
}

func handleGuest(h *Host, conn net.Conn) *Guest {
	g := new(Guest)
	g.conn = conn
	g.host = h
	g.logger = log.New(os.Stdout, fmt.Sprint("[Client ", conn.RemoteAddr(), "] "), log.LstdFlags)
	g.id = rand.Uint32()
	g.closed = false
	return g
}

func (g *Guest) Reader() {
	for {
		buf := make([]byte, 1024)
		n, err := g.conn.Read(buf)
		if err != nil {
			g.logger.Println(err)
			g.Close()
			g.logger.Println("closing Reader")
			return
		}
		g.host.outgoing <- libportal.Packet{libportal.Data, g.id, buf[:n]}
	}
}

func (g *Guest) Close() {
	if !g.closed {
		g.closed = true
		g.conn.Close()
		if !g.host.closed {
			g.host.outgoing <- libportal.Packet{libportal.GuestDisconnect, g.id, nil}
		}
	}
}
