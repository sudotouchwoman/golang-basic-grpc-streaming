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

// Represents connection produced by
// ConnectionFactory. Channel attributes represent
// read/write buffers. It is assumed that some underlying
// worker is listening on these channels.
type RawConnection struct {
	ReaderChan <-chan []byte
	WriterChan chan<- []byte
	ErrChan    <-chan error
	CloseHook  func() error
}

// Is used to establish connections with given
// properties or listing accessible connection targets.
type ConnectionFactory interface {
	NewWithProps(ConnectionProps) (RawConnection, error)
	ListAccessible() []ConnID
}

// Represents a connection requested by a particular client.
// Created by Provider. Single connection can be shared by several
// clients hence this entity. Stream of data is broadcasted to each proxy.
// All proxies share the same send channel
type ConnectionProxy interface {
	Recv(time.Duration) ([]byte, error)
	Send([]byte, time.Duration) error
	Close() error
}

// Manages connections requested by clients: opening and closing of
// connections, broadcasting and cleanup
type Provider interface {
	Open(ConnectionProps) (ConnectionProxy, error)
	Close(ConnID) error
	ListActive() []ConnID
	ListAccessible() []ConnID
}

var ErrAlreadyClosed = errors.New("connection is closed")
var ErrTimedOut = errors.New("request timed out")
var ErrIncompatibleProps = errors.New("already opened with incompatible props")
