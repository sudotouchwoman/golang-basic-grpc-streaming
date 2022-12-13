package connection

import (
	"context"
	"io"
	"sync"
	"time"
)

// ConnectionProxy implementation
// based on channels and context
// note that the underlying context
// should be managed by the provider
// and canceled in the cancelHook.
type ChannelConnectionProxy struct {
	SendLock   *sync.Mutex
	RecvLock   *sync.RWMutex
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
	// proxy.RecvLock.RLock()
	// if proxy.Done {
	// 	// to avoid unnesessary blocking
	// 	proxy.RecvLock.RUnlock()
	// 	return nil, io.EOF
	// }
	// proxy.RecvLock.RUnlock()

	select {
	case <-proxy.Ctx.Done():
		break
	default:
		select {
		case got, open := <-proxy.RecvChan:
			// make sure that this channel was opened
			// at the moment of call
			if open {
				return got, nil
			}
			break
		case <-time.After(t):
			return nil, ErrTimedOut
		}
	}
	return nil, io.EOF
}

// Ensure that this instance has not
// been canceled yet, then
// try to send data.
func (proxy *ChannelConnectionProxy) Send(p []byte, t time.Duration) error {
	// proxy.SendLock.Lock()
	// if proxy.Done {
	// 	// to avoid unnesessary send to closed channel
	// 	proxy.SendLock.Unlock()
	// 	return ErrAlreadyClosed
	// }
	// proxy.SendLock.Unlock()

	select {
	case <-proxy.Ctx.Done():
		return ErrAlreadyClosed
	default:
		select {
		case proxy.SendChan <- p:
			return nil
		case <-time.After(t):
			return ErrTimedOut
		}
	}
}

// Execute the cancel hook
func (proxy *ChannelConnectionProxy) Close() error {
	proxy.SendLock.Lock()
	if proxy.Done {
		// to avoid unnesessary send to closed channel
		proxy.SendLock.Unlock()
		return ErrAlreadyClosed
	}
	proxy.SendLock.Unlock()
	return proxy.CancelHook()
}
