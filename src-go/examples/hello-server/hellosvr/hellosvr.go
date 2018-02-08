package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/richardmillen/etude-2-net-patterns/src-go/check"
	"github.com/richardmillen/etude-2-net-patterns/src-go/examples/hello-server/input"
)

var closeBadClients = flag.Bool("close-bad-clients", true, "close the connection to client when invalid message is received, or return an error.")
var port = flag.Int("port", 5432, "port number to listen on")

var any = &fsm.Any{}

func newServer() (*netx.Service, error) {
	log.Println("configuring server states...")

	recvState := &fsm.State{
		Name: "receiving",
		Accepts: []fsm.Input{
			input.Hello,
			input.Hi,
		},
	}
	baseState := &fsm.State{
		Name:    "server base state",
		Accepts: []fsm.Input{any},
		Substates: []fsm.State{
			recvState,
		},
	}

	go func() {
		for {
			select {
			case r := <-baseState.Received(nil):
				buf := make([]byte, r.Input.Len())
				r.Input.Read(buf)
				log.Println("received invalid message:", buf)

				if *closeBadClients {
					log.Println("closing connection to client...")
					r.Close()
				} else {
					log.Println("returning error to client...")
					r.Write([]byte("invalid request"))
				}
			case <-baseState.Exited():
				log.Println("base state exited.")
				return
			}
		}
	}()

	log.Println("constructing hello server...")

	listener, err := netx.ListenTCP("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		return nil, err
	}

	return &netx.Service{
		Connector:    listener,
		InitialState: recvState,
	}, nil
}

func main() {
	flag.Parse()

	svc, err := newServer()
	check.Error(err)

	log.Println("server listening...")

	for {
		select {
		case r := <-svc.Received(nil):
			switch in := r.Input.(type) {
			case *fsm.String:
				log.Println("received:", in.From(r))
				r.Output.Write([]byte("world"))
			}
		case <-svc.Interrupt():
			log.Println("server interrupted.")
			return
		}
	}
}
