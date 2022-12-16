package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	loggerpb "github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/logger"
)

const defaultClientTimeout = 30 * time.Second

type LogStreamProps struct {
	Emitter     string
	Baudrate    uint32
	ReadTimeout int64
}

func (lp *LogStreamProps) ID() connection.ConnID {
	return connection.ConnID(lp.Emitter)
}

func (lp *LogStreamProps) Compatible(other connection.ConnectionProps) bool {
	switch other := other.(type) {
	case *LogStreamProps:
		return lp.Emitter == other.Emitter && lp.Baudrate == other.Baudrate
	default:
		return false
	}
}

type LogStreamerServer struct {
	loggerpb.UnimplementedLoggerServiceServer
	Provider connection.Provider
}

func (logSrv *LogStreamerServer) GetLogStream(req *loggerpb.LogStreamRequest, stream loggerpb.LoggerService_GetLogStreamServer) error {
	log.Printf("GetLogStream method invoked with request: %v\n", req)
	// some dummy emits at first, emulate time-consuming preprocessing
	for i := 0; i < 10; i++ {
		// create log record and emit it to the caller
		record := &loggerpb.LogRecord{
			Msg:     fmt.Sprintf("preparing for %s: left %d", req.Client, 10-i),
			Emitter: req.Emitter,
			Success: true,
		}
		if err := stream.Send(record); err != nil {
			log.Println(err)
			return err
		}
		// sleep for a bit
		time.Sleep(1e2 * time.Millisecond)
	}
	// in case
	if req.Timeout == 0 {
		log.Printf("no timeout specified, using default (%s)\n", defaultClientTimeout)
		req.Timeout = int64(defaultClientTimeout)
	}
	// after dummy messages sent,
	// try to create a connection to the given emitter
	props := LogStreamProps{
		req.Emitter,
		req.Baudrate,
		req.Timeout,
	}
	proxy, err := logSrv.Provider.Open(&props)
	if err != nil {
		log.Println("failed to open:", err)
		return stream.Send(&loggerpb.LogRecord{
			Msg:     fmt.Sprintf("failed to establish requested connection: %s", err),
			Emitter: req.Emitter,
			Success: false,
		})
	}
	// cleanup once connection is closed or reset
	defer proxy.Close()
	for {
		// quite a long timeout for now
		record, err := proxy.Recv(time.Duration(props.ReadTimeout))
		if err == io.EOF {
			// log.Println(req.Emitter, "connection exhausted")
			return stream.Send(&loggerpb.LogRecord{
				Msg:     "connection exhausted",
				Emitter: req.Emitter,
			})
		}
		if err != nil {
			log.Println("error while reading from proxy", err)
			return stream.Send(&loggerpb.LogRecord{
				Msg:     fmt.Sprintf("connection error: %s", err),
				Emitter: req.Emitter,
			})
		}
		if sendErr := stream.Send(&loggerpb.LogRecord{
			Msg:     string(record),
			Emitter: req.Emitter,
			Success: true,
		}); sendErr != nil {
			return sendErr
		}
	}
}

// Close the entire connection (for all clients)
func (logSrv *LogStreamerServer) StopLogStream(ctx context.Context, req *loggerpb.LogStreamRequest) (*loggerpb.LogRecord, error) {
	log.Printf("StopLogStream method invoked with request: %v\n", req)
	if err := logSrv.Provider.Close(connection.ConnID(req.Emitter)); err != nil {
		// connection has already been closed or something
		return &loggerpb.LogRecord{
			Msg:     fmt.Sprintf("Error on close: %s", err),
			Emitter: req.Emitter,
			Success: false,
		}, err
	}
	// OK
	return &loggerpb.LogRecord{
		Msg:     "Connection closed",
		Emitter: req.Emitter,
		Success: true,
	}, nil
}

func (logSrv *LogStreamerServer) GetAccessibleStreams(req *loggerpb.EmptyRequest, stream loggerpb.LoggerService_GetAccessibleStreamsServer) error {
	log.Printf("GetAccessibleStreams method invoked with request: %v\n", req)
	// some dummy emits at first, emulate time-consuming preprocessing
	// cleanup once connection is closed or reset
	ports := logSrv.Provider.ListAccessible()
	if len(ports) == 0 {
		return stream.Send(&loggerpb.LogRecord{})
	}
	for _, port := range ports {
		if err := stream.Send(&loggerpb.LogRecord{
			Emitter: string(port),
			Success: true,
		}); err != nil {
			return err
		}
	}
	return nil
}
