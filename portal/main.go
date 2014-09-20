package main

import (
	"fmt"
	"log"
	"net"
	"github.com/DeltaWhy/portal/libportal"
	"github.com/docopt/docopt-go"
)

func main() {
	const usage = `TCP tunneling client.

Usage:
    portal [options] -s <server-addr> -t <target-addr>
    portal (-h | --help | --version)

Options:
    -h, --help                  Show this screen
    --version                   Show version
    -s, --server <server-addr>  Address of portald server to connect to
    -t, --target <target-addr>  Address of receiving server
    -m, --meta <meta>           Extra information for server`

	args, err := docopt.Parse(usage, nil, true, "portal 0.1", false)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println(args)
	log.SetPrefix("[portal] ")
	client, err := net.Dial("tcp", args["--server"].(string))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected from ", client.LocalAddr())
	t := NewTunnel(client, args["--target"].(string), args["--meta"].(string))
	/*stop := make(chan struct{})
	go reader(stop, client)
	go writer(stop, client)
	_ = <-stop*/
	handleTunnel(t)
	log.Println("exited cleanly")
}

func handleTunnel(t *Tunnel) {
	gs := make(map[uint32]*Guest)
	for pkt := range t.incoming {
		if t.state == Init {
			if pkt.Kind != libportal.AuthReq {
				t.logger.Println("expected AuthReq")
				t.Close()
				return
			}
			fmt.Print("AuthReq ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
			t.state = Authing
			t.outgoing <- libportal.Packet{libportal.AuthResp, 0, nil} //TODO real auth
		} else if t.state == Authing {
			switch pkt.Kind {
			case libportal.OK:
				t.state = Authed
				t.outgoing <- libportal.Packet{libportal.GameMeta, 0, []byte(t.meta)}
			case libportal.Error:
				fmt.Print("Error ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
				t.Close()
				return
			default:
				t.logger.Println("expected OK or Error")
				t.Close()
				return
			}
		} else if t.state == Authed {
			switch pkt.Kind {
			case libportal.OK:
				t.state = Ready
				fmt.Print("OK ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
			case libportal.Error:
				fmt.Print("Error ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
				t.Close()
				return
			default:
				t.logger.Println("expected OK or Error")
				t.Close()
				return
			}
		} else if t.state == Ready {
			switch pkt.Kind {
			case libportal.Ping:
			case libportal.Error:
				fmt.Print("Error ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
				t.Close()
				return
			case libportal.GuestConnect:
				fmt.Print("GuestConnect ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
				conn, err := net.Dial("tcp", t.target)
				if err != nil {
					t.logger.Println("error connecting to target: ", err)
					t.outgoing <- libportal.Packet{libportal.GuestDisconnect, pkt.ConnId, nil}
				} else {
					g := NewGuest(t, conn, pkt.ConnId)
					gs[g.id] = g
				}
			case libportal.GuestDisconnect:
				fmt.Print("GuestDisconnect ", pkt.ConnId, ": ", string(pkt.Payload), "\n")
				if gs[pkt.ConnId] != nil {
					gs[pkt.ConnId].Close()
					delete(gs, pkt.ConnId)
				}
			case libportal.Data:
				fmt.Print("DATA ", pkt.ConnId, ": ", len(pkt.Payload), "\n")
				if gs[pkt.ConnId] != nil {
					_, err := gs[pkt.ConnId].conn.Write(pkt.Payload)
					if err != nil {
						t.logger.Println(err)
						gs[pkt.ConnId].Close()
						delete(gs, pkt.ConnId)
						t.outgoing <- libportal.Packet{libportal.GuestDisconnect, pkt.ConnId, nil}
					}
				}
			default:
				t.logger.Println("unexpected packet Kind=", pkt.Kind, " ConnId=", pkt.ConnId, ": ", string(pkt.Payload))
				t.Close()
				return
			}
		} else {
			t.logger.Println("unknown state")
			t.Close()
			return
		}
	}
}
