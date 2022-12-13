package connection

import (
	"context"
	"io"
	"log"
	"time"
)

// ConnectionProxy implementation
// based on channels and context
// note that the underlying context
// should be managed by the provider
// and canceled in the cancelHook.
type ChannelConnectionProxy struct {
	Ctx        context.Context
	SendChan   chan<- []byte
	RecvChan   <-chan []byte
	CancelHook func() error
	Done       bool
}

// Ensure that this instance has not
// been canceled yet, then
// try to recieve data.
func (proxy *ChannelConnectionProxy) Recv(t time.Duration) ([]byte, error) {
	select {
	case <-proxy.Ctx.Done():
		log.Println("proxy context done")
		break
	case got, open := <-proxy.RecvChan:
		// make sure that this channel was opened
		// at the moment of call
		if open {
			log.Println("proxy recieved:", string(got))
			return got, nil
		}
		break
	case <-time.After(t):
		log.Println("proxy recv timed out")
		return nil, ErrTimedOut
	}
	log.Println("proxy already closed")
	return nil, io.EOF
}

// Ensure that this instance has not
// been canceled yet, then
// try to send data.
func (proxy *ChannelConnectionProxy) Send(p []byte, t time.Duration) error {
	select {
	case proxy.SendChan <- p:
		return nil
	case <-proxy.Ctx.Done():
		return ErrAlreadyClosed
	case <-time.After(t):
		return ErrTimedOut
	}
}

// Execute the cancel hook
func (proxy *ChannelConnectionProxy) Close() error {
	if proxy.Done {
		log.Println("proxy.Close(): already closed")
		return ErrAlreadyClosed
	}
	log.Println("cancels proxy")
	return proxy.CancelHook()
}
