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
	addr := "localhost:8080"
	flag.StringVar(&addr, "-a", addr, "gRPC Dial Address")
	flag.Parse()

	cc, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect: %e", err)
	}
	defer cc.Close()
	ListenForLogs(loggerpb.NewLoggerServiceClient(cc))
}

func ListenForLogs(cl loggerpb.LoggerServiceClient) {
	log.Println("connected to gRPC streaming server, starts polling")
	req := &loggerpb.LogStreamRequest{
		Emitter:  "esp32",
		Client:   strconv.Itoa(os.Getpid()),
		Baudrate: 115200,
	}

	logStream, err := cl.GetLogStream(context.Background(), req)
	if err != nil {
		log.Fatalf("error while querying server: %e", err)
	}
	for {
		msg, err := logStream.Recv()
		if err == io.EOF {
			log.Println("streaming finished")
			break
		}
		if err != nil {
			log.Fatalf("error during stream read: %e", err)
		}
		log.Printf("[%s] - (%s)", msg.Emitter, msg.Msg)
	}
	log.Println("client finished")
}
