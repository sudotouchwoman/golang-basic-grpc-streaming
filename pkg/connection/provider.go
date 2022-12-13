package connection

import (
	"errors"
	"time"
)

type ConnID string

type ConnectionProps interface {
	ID() ConnID
	Compatible(ConnectionProps) bool
}

type RawConnection struct {
	ReaderChan <-chan []byte
	WriterChan chan<- []byte
	Err        error
	CloseHook  func() error
}

type ConnectionFactory interface {
	NewWithProps(ConnectionProps) RawConnection
}

type ConnectionProxy interface {
	Recv(time.Duration) ([]byte, error)
	Send([]byte, time.Duration) error
	Close() error
}

type Provider interface {
	Open(ConnectionProps) (ConnectionProxy, error)
	Close(ConnID) error
	ListActive() []ConnID
}

var ErrAlreadyClosed = errors.New("connection is closed already")
var ErrTimedOut = errors.New("request timed out")
var ErrIncompatibleProps = errors.New("already opened with incompatible props")
