package main

import (
	"errors"
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
	"time"
)

type Server struct {
	streamChan chan *smux.Stream
	die        chan struct{}
}

func privateServer(srv *Server) {
	listen, err := net.Listen("tcp", "0.0.0.0:9001")
	if err != nil {
		log.Fatalln("listen error", err)
	}
	log.Println("starting private conn")
	defer listen.Close()
	tcpListen := listen.(*net.TCPListener)
	for {
		tc, err := tcpListen.AcceptTCP()
		if err != nil {
			log.Println("accept failed", err)
			return
		}
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)
		log.Println("new from client")
		go privateConn(srv, tc)
	}
}

func proxyServer(srv *Server) {
	listen, err := net.Listen("tcp", "0.0.0.0:9000")
	if err != nil {
		log.Fatalln("listen error", err)
	}
	log.Println("starting proxy conn")
	defer listen.Close()
	tcpListen := listen.(*net.TCPListener)
	for {
		tc, err := tcpListen.AcceptTCP()
		if err != nil {
			log.Println("accept failed", err)
			return
		}
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)
		go proxyConn(srv, tc)
	}
}

func proxyConn(srv *Server, tc *net.TCPConn) error {
	var deadline <-chan time.Time
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	deadline = timer.C
	defer tc.Close()

	select {
	case prv := <-srv.streamChan:
		defer prv.Close()

		// start tunnel
		p1die := make(chan struct{})
		go func() { io.Copy(prv, tc); close(p1die) }()

		p2die := make(chan struct{})
		go func() { io.Copy(tc, prv); close(p2die) }()

		// wait for tunnel termination
		select {
		case <-p1die:
		case <-p2die:
		}
	case <-deadline:
		return errors.New("timeout")
	case <-srv.die:
		return errors.New("die")
	}

	return nil
}

func privateConn(srv *Server, tc *net.TCPConn) error {
	session, err := smux.Client(tc, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	for {
		if err = loopPrivateConn(srv, session); err != nil {
			return err
		}
	}
}

func loopPrivateConn(srv *Server, session *smux.Session) error {
	stream, err := session.OpenStream()
	if err != nil {
		return err
	}
	log.Println("open client stream ok")
	select {
	case srv.streamChan <- stream:
	case <-srv.die:
		stream.Close()
		return errors.New("server die")
	}

	return nil
}

func serverLoop(srv *Server) {
	select {}
}

func main() {
	srv := &Server{
		streamChan: make(chan *smux.Stream),
		die:        make(chan struct{}),
	}
	go proxyServer(srv)
	go privateServer(srv)
	for {
		serverLoop(srv)
	}
}
