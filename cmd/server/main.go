package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	addr := "localhost:8080"
	flag.StringVar(&addr, "addr", addr, "gRPC Dial Address")
	flag.Parse()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %e", err)
	}

	rootCtx, rootCtxCancel := context.WithCancel(context.Background())
	defer rootCtxCancel()

	// create providers for log streaming
	provider := connection.NewConnctionProvider(
		rootCtx, server.NewTickerFactory(rootCtx, 5*time.Second),
	)

	// create and register server
	s := grpc.NewServer()
	loggerpb.RegisterLoggerServiceServer(s, &server.LogStreamerServer{Provider: provider})
	log.Println("gRPC server starting...")

	if err = s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %e", err)
	}
}
