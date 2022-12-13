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

func main() {
	addr, emitter := "localhost:8080", "esp32"
	flag.StringVar(&emitter, "e", emitter, "Emitter name")
	flag.StringVar(&addr, "addr", addr, "gRPC Dial Address")
	flag.Parse()

	cc, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect: %s", err)
	}
	defer cc.Close()
	ListenForLogs(loggerpb.NewLoggerServiceClient(cc), emitter)
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
