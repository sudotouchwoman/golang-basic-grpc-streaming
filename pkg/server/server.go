package server

import (
	"fmt"
	"log"
	"time"

	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
)

type LogStreamerServer struct {
	loggerpb.UnimplementedLoggerServiceServer
}

func (logSrv *LogStreamerServer) GetLogStream(req *loggerpb.LogStreamRequest, stream loggerpb.LoggerService_GetLogStreamServer) error {
	log.Printf("GetLogStream method invoked with request: %v\n", req)
	for i := 0; i < 10; i++ {
		// create log record and emit it to the caller
		record := &loggerpb.LogRecord{
			Msg:     fmt.Sprintf("monitoring %s: chunk %d", req.Client, i),
			Emitter: req.Emitter,
			Success: true,
		}
		if err := stream.Send(record); err != nil {
			log.Println(err)
			return err
		}
		// sleep for a bit
		time.Sleep(1e3 * time.Millisecond)
	}
	return nil
}
