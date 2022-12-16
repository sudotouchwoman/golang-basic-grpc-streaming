package main

import (
	"context"
	"flag"
	"log"
	"sync"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/serial"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
)

func main() {
	port := "/dev/ttyUSB0"
	flag.StringVar(&port, "com", port, "Serial port name")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	factory := serial.SerialConnectionFactory{
		Mu:  &sync.RWMutex{},
		Ctx: ctx,
	}

	conn, err := factory.NewWithProps(&server.LogStreamProps{
		Emitter:  port,
		Baudrate: 115200,
	})
	if err != nil {
		log.Fatalln(err)
	}
	for range conn.ReaderChan {

	}
}
