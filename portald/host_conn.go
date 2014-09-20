package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
	"github.com/DeltaWhy/portal/libportal"
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
	incoming chan libportal.Packet
	outgoing chan libportal.Packet
	guests chan *Guest
	closed bool
}

func handleHost(conn net.Conn) *Host {
	h := new(Host)
	h.conn = conn
	h.logger = log.New(os.Stdout, fmt.Sprint("[", conn.RemoteAddr(), "] "), log.LstdFlags)
	h.state = Unauthed
	h.incoming = make(chan libportal.Packet)
	h.outgoing = make(chan libportal.Packet)
	h.guests = make(chan *Guest)
	h.closed = false
	go h.Reader()
	go h.Writer()
	go hostSetup(h)
	//time.Sleep(15*time.Second)
	//h.Close()
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

	for {
		h.conn.SetReadDeadline(time.Now().Add(30*time.Second))
		var header libportal.PacketHeader
		err := binary.Read(h.conn, binary.BigEndian, &header)
		if err != nil {
			h.logger.Println(err)
			h.Close()
			h.logger.Println("closing Reader")
			return
		}
		payload := make([]byte, header.Length)
		_, err = io.ReadFull(h.conn, payload)
		if err != nil {
			h.logger.Println(err)
			h.Close()
			h.logger.Println("closing Reader")
			return
		}
		h.incoming <- libportal.Packet{Kind: header.Kind, ConnId: header.ConnId, Payload: payload}
	}
}

// reads the outgoing channel and writes to the socket
func (h *Host) Writer() {
	for message := range h.outgoing {
		h.conn.SetWriteDeadline(time.Now().Add(30*time.Second))
		err := binary.Write(h.conn, binary.BigEndian, libportal.Header(message))
		if err != nil {
			h.logger.Println(err)
			h.Close()
			h.logger.Println("closing Writer")
			return
		}
		_, err = h.conn.Write(message.Payload)
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

func (h *Host) Pinger() {
	defer func() {
		recover()
	}()

	for {
		time.Sleep(20*time.Second)
		h.outgoing <- libportal.Packet{libportal.Ping, 0, nil}
	}
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
	h.outgoing <- libportal.Packet{libportal.AuthReq, 0, nil}
	h.state = Authing

	resp, ok := <-h.incoming
	if !ok {
		h.logger.Println("closed before authing")
		return
	}
	// TODO check response
	if resp.Kind != libportal.AuthResp {
		h.outgoing <- libportal.Err("expected AuthResp")
		time.Sleep(time.Second)
		h.Close()
		return
	}
	h.state = Authed

	h.logger.Println("auth response:", string(resp.Payload))

	h.outgoing <- libportal.Okay("auth OK")

	resp, ok = <-h.incoming
	if !ok {
		h.logger.Println("closed before gameMeta")
		return
	}
	if resp.Kind != libportal.GameMeta {
		h.outgoing <- libportal.Err("expected GameMeta")
		time.Sleep(time.Second)
		h.Close()
		return
	}
	h.logger.Println("gameMeta:", string(resp.Payload))

	var err error
	h.outside, err = net.Listen("tcp", ":")
	if err != nil {
		h.logger.Println(err)
		h.outgoing <- libportal.Err("error opening outside port")
		h.Close()
		return
	}
	h.outgoing <- libportal.Okay(fmt.Sprint("opened outside ", h.outside.Addr()))
	go h.Listener()
	go h.Pinger()
	h.state = Ready

	gs := make(map[uint32]*Guest)
	handlerLoop:
	for {
		select {
		case pkt, ok := <-h.incoming:
			if !ok {
				break handlerLoop
			}
			switch pkt.Kind {
			case libportal.Ping:
			case libportal.Data:
				h.logger.Println("DATA ", pkt.ConnId, ": ", len(pkt.Payload))
				if gs[pkt.ConnId] != nil {
					_, err := gs[pkt.ConnId].conn.Write(pkt.Payload)
					if err != nil {
						h.logger.Println(err)
						gs[pkt.ConnId].Close()
						delete(gs, pkt.ConnId)
						h.outgoing <- libportal.Packet{libportal.GuestDisconnect, pkt.ConnId, nil}
					}
				} else {
					h.outgoing <- libportal.Packet{libportal.GuestDisconnect, pkt.ConnId, nil}
				}
			case libportal.GuestDisconnect:
				if gs[pkt.ConnId] != nil {
					gs[pkt.ConnId].Close()
					delete(gs, pkt.ConnId)
				}
			default:
				h.logger.Println("unknown packet from host")
			}
		case g, ok := <-h.guests:
			if ok {
				h.logger.Println("handler got guest")
				gs[g.id] = g
				h.outgoing <- libportal.Packet{libportal.GuestConnect, g.id, nil}
				go g.Reader()
			} else {
				break handlerLoop
			}
		}
	}
	for _, g := range gs {
		g.Close()
	}
}
