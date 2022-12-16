package main

import (
	"context"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
)

type LoggerServiceClient struct {
	loggerpb.LoggerServiceClient
}

func (cl *LoggerServiceClient) ListenForLogs(emitter string) {
	log.Println("connected to gRPC streaming server, starts polling")
	defer log.Println("client finished")
	req := &loggerpb.LogStreamRequest{
		Emitter:  emitter,
		Client:   strconv.Itoa(os.Getpid()),
		Baudrate: 115200,
	}

	logStream, err := cl.GetLogStream(context.Background(), req)
	if err != nil {
		log.Fatalf("error while querying server: %s", err)
	}
	for {
		msg, err := logStream.Recv()
		if err == io.EOF {
			log.Println("streaming finished")
			break
		}
		if err != nil {
			log.Fatalf("stream read: %s", err)
		}
		log.Printf("[%s] - (%s)", msg.Emitter, msg.Msg)
	}
}

func (cl *LoggerServiceClient) CloseLogStream(emitter string) {
	// connects to gRPC backend and unconditionally closes
	// connection to specified emitter (data source)
	log.Println("connected to gRPC server, aims to close emitter")
	defer log.Println("client finished")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &loggerpb.LogStreamRequest{
		Emitter:  emitter,
		Client:   strconv.Itoa(os.Getpid()),
		Baudrate: 115200, // hardcoded for now
	}

	record, err := cl.StopLogStream(ctx, req)
	if err != nil {
		log.Fatal("stop log stream:", err)
	}
	log.Printf("[%s] - (%s)", record.Emitter, record.Msg)
}

func (cl *LoggerServiceClient) AccessibleStreams() {
	log.Println("connected to gRPC streaming server, starts polling")
	defer log.Println("client finished")

	logStream, err := cl.GetAccessibleStreams(context.Background(), &loggerpb.EmptyRequest{})
	if err != nil {
		log.Fatalf("error while querying server: %s", err)
	}
	ports := []string{}
	for {
		msg, err := logStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("stream read: %s", err)
		}
		if !msg.Success {
			log.Println("No open ports")
			return
		}
		ports = append(ports, msg.Emitter)
	}
	log.Println("Open Ports: ", strings.Join(ports, ", "))
}
