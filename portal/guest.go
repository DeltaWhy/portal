package main

import (
	"log"
	"net"
	"github.com/DeltaWhy/portal/libportal"
)

type Guest struct {
	conn net.Conn
	tunnel *Tunnel
	id uint32
	closed bool
}

func NewGuest(tunnel *Tunnel, conn net.Conn, id uint32) *Guest {
	g := new(Guest)
	g.conn = conn
	g.tunnel = tunnel
	g.id = id
	g.closed = false
	go g.Reader()
	return g
}

func (g *Guest) Reader() {
	for {
		buf := make([]byte, 1024)
		n, err := g.conn.Read(buf)
		if err != nil {
			log.Println(err)
			g.Close()
			return
		}
		log.Println("guest received ", n)
		g.tunnel.outgoing <- libportal.Packet{libportal.Data, g.id, buf[:n]}
	}
}

func (g *Guest) Close() {
	if !g.closed {
		g.closed = true
		g.conn.Close()
	}
}
