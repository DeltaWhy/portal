package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
	"github.com/docopt/docopt-go"
	"github.com/fzzy/radix/redis"
)

var redis_client *redis.Client
var redis_prefix string = "netherhub"

func main() {
	const usage = `TCP tunneling server.

Usage:
    portald [options]
    portald (-h | --help | --version)

Options:
    -h, --help                  Show this screen
    --version                   Show version
    -b, --bind <bind-addr>      Interface to bind to
    -p, --port <port>           Port to bind to
    -r, --redis <redis-server>  Redis server address
    --prefix <redis-prefix>     Prefix for Redis keys`

	args, err := docopt.Parse(usage, nil, true, "portald 0.1", false)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println(args)

	rand.Seed(time.Now().UTC().UnixNano())
	log.SetPrefix("[portald] ")
	addr, port := "", "9000"
	if args["--bind"] != nil {
		addr = args["--bind"].(string)
	}
	if args["--port"] != nil {
		port = args["--port"].(string)
	}
	srv, err := net.Listen("tcp4", fmt.Sprintf("%s:%s", addr, port))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on ", srv.Addr())

	redis_addr := "localhost:6379"
	if args["--redis"] != nil {
		redis_addr = args["--redis"].(string)
	}
	if args["--prefix"] != nil {
		redis_prefix = args["--prefix"].(string)
	}
	redis_client, err = redis.Dial("tcp4", redis_addr)
	if err != nil {
		log.Fatal(err)
	}

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
