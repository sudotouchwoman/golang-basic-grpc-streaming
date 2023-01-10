package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/serial"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	addr := "localhost:8080"
	flag.StringVar(&addr, "addr", addr, "gRPC Dial Address")
	flag.Parse()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}

	rootCtx, rootCtxCancel := context.WithCancel(context.Background())
	defer rootCtxCancel()

	// create providers for log streaming
	provider := connection.NewConnectionProvider(
		rootCtx, &serial.SerialConnectionFactory{
			Mu:  &sync.RWMutex{},
			Ctx: rootCtx,
		},
	)

	// create and register server
	s := grpc.NewServer()
	loggerpb.RegisterLoggerServiceServer(s, &server.LogStreamerServer{Provider: provider})
	log.Println("gRPC server starting...")

	if err = s.Serve(listener); err != nil {
		log.Fatalln("failed to serve:", err)
	}
}
