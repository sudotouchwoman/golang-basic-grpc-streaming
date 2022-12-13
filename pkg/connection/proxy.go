package connection

import (
	"context"
	"time"
)

// ConnectionProxy implementation
// based on channels and context
// note that the underlying context
// should be managed by the provider
// and canceled in the cancelHook.
type ChannelConnectionProxy struct {
	ConnectionProxy
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
	case got := <-proxy.RecvChan:
		return got, nil
	case <-proxy.Ctx.Done():
		return nil, ErrAlreadyClosed
	case <-time.After(t):
		break
	}
	return nil, ErrTimedOut
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
		break
	}
	return ErrTimedOut
}

// Execute the cancel hook
func (proxy *ChannelConnectionProxy) Close() error {
	if proxy.Done {
		return ErrAlreadyClosed
	}
	return proxy.CancelHook()
}
