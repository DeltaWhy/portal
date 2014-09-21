package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"time"
	"github.com/DeltaWhy/portal/libportal"
)

type TunnelState int

const (
	Init TunnelState = iota
	Authing
	Authed
	Ready
)

type Tunnel struct {
	conn net.Conn
	logger *log.Logger
	state TunnelState
	auth string
	meta string
	target string
	incoming chan libportal.Packet
	outgoing chan libportal.Packet
	closed bool
}

func NewTunnel(conn net.Conn, target string, auth string, meta string) *Tunnel {
	t := new(Tunnel)
	t.conn = conn
	t.logger = log.New(os.Stdout, "[portal] ", log.LstdFlags)
	t.state = Init
	t.auth = auth
	t.meta = meta
	t.target = target
	t.incoming = make(chan libportal.Packet)
	t.outgoing = make(chan libportal.Packet)
	t.closed = false
	go t.Reader()
	go t.Writer()
	go t.Pinger()
	return t
}

// reads the socket and writes to the incoming channel
func (t *Tunnel) Reader() {
	// need to recover if the incoming channel is closed
	defer func() {
		if r := recover(); r != nil {
			t.logger.Println("incoming channel was closed")
		}
	}()

	for {
		t.conn.SetReadDeadline(time.Now().Add(30*time.Second))
		var header libportal.PacketHeader
		err := binary.Read(t.conn, binary.BigEndian, &header)
		if err != nil {
			t.logger.Println(err)
			t.Close()
			t.logger.Println("closing Reader")
			return
		}
		payload := make([]byte, header.Length)
		_, err = io.ReadFull(t.conn, payload)
		if err != nil {
			t.logger.Println(err)
			t.Close()
			t.logger.Println("closing Reader")
			return
		}
		t.incoming <- libportal.Packet{Kind: header.Kind, ConnId: header.ConnId, Payload: payload}
	}
}

// reads the outgoing channel and writes to the socket
func (t *Tunnel) Writer() {
	for message := range t.outgoing {
		t.conn.SetWriteDeadline(time.Now().Add(30*time.Second))
		err := binary.Write(t.conn, binary.BigEndian, libportal.Header(message))
		if err != nil {
			t.logger.Println(err)
			t.Close()
			t.logger.Println("closing Writer")
			return
		}
		_, err = t.conn.Write(message.Payload)
		if err != nil {
			t.logger.Println(err)
			t.Close()
			t.logger.Println("closing Writer")
			return
		}
	}
	t.logger.Println("closing Writer")
}

func (t *Tunnel) Pinger() {
	defer func() {
		recover()
	}()

	for {
		time.Sleep(20*time.Second)
		t.outgoing <- libportal.Packet{libportal.Ping, 0, nil}
	}
}

func (t *Tunnel) Close() {
	if !t.closed {
		t.closed = true
		close(t.incoming)
		close(t.outgoing)
		t.conn.Close()
	}
}
