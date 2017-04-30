package main

import (
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
)

type Client struct {
	die chan struct{}
}

func proxyClient(cli *Client) error {
	conn, err := net.Dial("tcp", "127.0.0.1:9001")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	session, err := smux.Server(conn, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return err
		}
		log.Println("accept new stream")
		go processProxyStream(cli, stream)
	}
}

func processProxyStream(cli *Client, stream *smux.Stream) error {
	defer stream.Close()
	conn, err := net.Dial("tcp", "192.168.6.1:80")
	if err != nil {
		log.Println(err)
		return err
	}
	defer conn.Close()
	p1die := make(chan struct{})
	go func() { io.Copy(stream, conn); close(p1die) }()

	p2die := make(chan struct{})
	go func() { io.Copy(conn, stream); close(p2die) }()

	select {
	case <-p1die:
	case <-p2die:
	}
	return nil
}

func main() {
	cli := &Client{
		die: make(chan struct{}),
	}
	proxyClient(cli)
}
