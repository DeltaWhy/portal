package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
	"github.com/DeltaWhy/portal/libportal"
)

func main() {
	log.SetPrefix("[portal] ")
	client, err := net.Dial("tcp", "127.0.0.1:9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected from ", client.LocalAddr())
	stop := make(chan struct{})
	go reader(stop, client)
	go writer(stop, client)
	_ = <-stop
	log.Println("exited cleanly")
}

func reader(stop chan struct{}, conn net.Conn) {
	for {
		conn.SetReadDeadline(time.Now().Add(30*time.Second))
		var header libportal.PacketHeader
		err := binary.Read(conn, binary.BigEndian, &header)
		if err != nil {
			log.Println(err)
			conn.Close()
			log.Println("closing Reader")
			stop <- struct{}{}
			return
		} else {
			log.Println("got header")
		}
		payload := make([]byte, header.Length)
		_, err = io.ReadFull(conn, payload)
		if err != nil {
			log.Println(err)
			conn.Close()
			log.Println("closing Reader")
			stop <- struct{}{}
			return
		}
		fmt.Print("DATA ", header.ConnId, ": ", string(payload))
	}
}

func writer(stop chan struct{}, conn net.Conn) {
	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			log.Println(err)
			conn.Close()
			stop <- struct{}{}
			log.Println("closing Writer")
			return
		}
		packet := libportal.StrPacket(line)
		err = binary.Write(conn, binary.BigEndian, libportal.Header(packet))
		if err != nil {
			log.Println(err)
			conn.Close()
			stop <- struct{}{}
			log.Println("closing Writer")
			return
		}
		_, err = conn.Write(packet.Payload)
		if err != nil {
			log.Println(err)
			conn.Close()
			stop <- struct{}{}
			log.Println("closing Writer")
			return
		}
	}
}
