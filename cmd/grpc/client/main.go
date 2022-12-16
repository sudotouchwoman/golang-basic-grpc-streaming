package main

import (
	"flag"
	"log"

	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ActionRead   = "read"
	ActionCancel = "cancel"
	ActionList   = "list"
)

// Basic cli gRPC client implementation for log streamer
// connects and performs single action (list/read/cancel)
// further work involves leveraging current API to support
// timestamps, data persistence and web frontend
func main() {
	addr, emitter := "localhost:8080", "/dev/ttyUSB0"
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

	cl := LoggerServiceClient{loggerpb.NewLoggerServiceClient(cc)}
	switch action {
	case ActionRead:
		cl.ListenForLogs(emitter)
		return
	case ActionCancel:
		cl.CloseLogStream(emitter)
		return
	case ActionList:
		cl.AccessibleStreams()
		return
	default:
		log.Fatal("unknown action:", action)
	}
}
