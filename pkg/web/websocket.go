package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
)

const (
	MethodRead     = "read"
	MethodDiscover = "discover"
)

var (
	ErrMethodInvaid = errors.New("this method is not supported")
	ErrContextDone  = errors.New("socket handlers factory context done")
)

type LogSockClientFactory struct {
	Ctx         context.Context
	Provider    connection.Provider
	ReadTimeout time.Duration
}

// This factory is still somewhat static, it might be tricky
// e.g. to update timeout value in runtime yet for now
// this does not matter that much
func (f *LogSockClientFactory) New(conn *websocket.Conn) SockHandler {
	select {
	case <-f.Ctx.Done():
		return nil
	default:
		return &LogSockClient{
			Wg:         &sync.WaitGroup{},
			Ctx:        f.Ctx,
			Conn:       conn,
			WriterChan: make(chan []byte, 10),
			Timeout:    f.ReadTimeout,
			Provider:   f.Provider,
		}
	}
}

type LogSockClient struct {
	Wg         *sync.WaitGroup
	Ctx        context.Context
	Conn       *websocket.Conn
	WriterChan chan []byte
	Timeout    time.Duration
	Provider   connection.Provider
}

// This method blocks until client disconnects. It listens
// for client requests and launches command handlers in
// separate goroutines. Once client disconnects, this method
// is still blocked on defer (waiting for all subroutines to finish writing
// so that the shared channel could be closed)
// I decided that blocking on context might be a stupid idea given
// that handler is basically stateless and interrupting it is safe.
func (cl *LogSockClient) Read() {
	defer func() {
		cl.Wg.Wait()
		close(cl.WriterChan)
		log.Println("closed send chan")
	}()

	buf := SerialRequest{}
	// child goroutines might block forever thus it might
	// be a good idea to tell them to stop once this method completes
	peersContext, peersContextCancel := context.WithCancel(cl.Ctx)
	defer peersContextCancel()

	// loop through messages from client
	// respond with errors on bad payload
	for {
		// update timeout on each read
		cl.Conn.SetReadDeadline(time.Now().Add(cl.Timeout))
		// wait for next frame to arrive, return on errors
		// and skip frame on json decode errors
		_, r, err := cl.Conn.NextReader()
		if err != nil {
			log.Println("next reader err", err)
			return
		}
		if err = json.NewDecoder(r).Decode(&buf); err != nil {
			log.Println("bad read from client:", err)
			cl.WriterChan <- JsonifyError(err, buf.Serial)
			continue
		}
		// process parsed message, try to establish connection
		go cl.Handle(peersContext, buf)
	}
}

// This method is intended to be launched in a separate
// goroutine inside http handler body just before blocking on Read().
// Exits once writer channel is closed.
func (cl *LogSockClient) Write() {
	for msg := range cl.WriterChan {
		if err := cl.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Println("err send:", err)
			return
		}
	}
}

// Based on request method type, delegates processing to
// other methods. Note that this method blocks WaitGroup to prevent
// Read() method from closing the channel handlers might write to.
func (cl *LogSockClient) Handle(Ctx context.Context, r SerialRequest) {
	cl.Wg.Add(1)
	defer cl.Wg.Done()

	switch r.Method {
	case MethodRead:
		cl.handleRead(Ctx, r)
	case MethodDiscover:
		cl.handleSerialDiscover()
	default:
		cl.WriterChan <- JsonifyError(ErrMethodInvaid, "")
	}
}

func (cl *LogSockClient) handleSerialDiscover() {
	discovered := cl.Provider.ListAccessible()
	msg, err := json.Marshal(&DiscoverySerialMessage{
		Serials: discovered,
		Iat:     time.Now(),
	})
	if err != nil {
		log.Fatalln(err)
	}
	cl.WriterChan <- msg
}

// used to write messages associated with this request
func (cl *LogSockClient) handleRead(Ctx context.Context, r SerialRequest) {
	buf := BasicSerialMessage{
		Serial: r.Serial,
	}
	proxy, err := cl.Provider.Open(&server.LogStreamProps{
		Emitter:     r.Serial,
		Baudrate:    r.Baudrate,
		ReadTimeout: r.Timeout,
	})
	if err != nil {
		cl.WriterChan <- JsonifyError(err, r.Serial)
		return
	}
	defer proxy.Close()

	for {
		select {
		case <-Ctx.Done():
			log.Println(buf.Serial, "peer disconnected")
			return
		default:
			// block on recieve
			// inform user if any error is encountered
			recv, err := proxy.Recv(time.Duration(r.Timeout) * time.Second)
			if err != nil {
				cl.WriterChan <- JsonifyError(err, r.Serial)
				return
			}
			// pass obtained record
			buf.Message = recv
			buf.Iat = time.Now()
			if msg, err := json.Marshal(&buf); err == nil {
				cl.WriterChan <- msg
				continue
			}
			return
		}
	}
}