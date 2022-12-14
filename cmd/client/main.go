package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"strconv"

	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ActionRead   = "read"
	ActionCancel = "cancel"
)

func main() {
	addr, emitter := "localhost:8080", "esp32"
	action := ActionRead
	flag.StringVar(&emitter, "e", emitter, "Emitter name")
	flag.StringVar(&addr, "addr", addr, "gRPC Dial Address")
	flag.StringVar(&action, "action", action, "gRPC method to call")
	flag.Parse()

	cc, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect: %s", err)
	}
	defer cc.Close()
	switch action {
	case ActionRead:
		ListenForLogs(loggerpb.NewLoggerServiceClient(cc), emitter)
		return
	case ActionCancel:
		CloseLogStream(loggerpb.NewLoggerServiceClient(cc), emitter)
		return
	default:
		log.Fatal("unknown action:", action)
	}
}

func ListenForLogs(cl loggerpb.LoggerServiceClient, emitter string) {
	log.Println("connected to gRPC streaming server, starts polling")
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
	log.Println("client finished")
}

func CloseLogStream(cl loggerpb.LoggerServiceClient, emitter string) {
	// first draft
	log.Println("connected to gRPC server, aims to close emitter")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &loggerpb.LogStreamRequest{
		Emitter:  emitter,
		Client:   strconv.Itoa(os.Getpid()),
		Baudrate: 115200,
	}

	record, err := cl.StopLogStream(ctx, req)
	if err != nil {
		log.Fatal("stop log stream:", err)
	}
	log.Printf("[%s] - (%s)", record.Emitter, record.Msg)
	log.Println("client finished")
}
