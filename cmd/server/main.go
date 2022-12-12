package main

import (
	"flag"
	"log"
	"net"

	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	addr := "localhost:8080"
	flag.StringVar(&addr, "-a", addr, "gRPC Dial Address")
	flag.Parse()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %e", err)
	}

	s := grpc.NewServer()
	loggerpb.RegisterLoggerServiceServer(s, &server.LogStreamerServer{})
	log.Println("gRPC server starting...")

	if err = s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %e", err)
	}
}
