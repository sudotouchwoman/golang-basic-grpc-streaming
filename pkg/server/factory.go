package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
)

type TickerFactory struct {
	Lock   *sync.RWMutex
	Ctx    context.Context
	Period time.Duration
	Active map[connection.ConnID]bool
	Error  error
}

func NewTickerFactory(ctx context.Context, period time.Duration) *TickerFactory {
	return &TickerFactory{
		Lock:   &sync.RWMutex{},
		Ctx:    ctx,
		Period: period,
		Active: map[connection.ConnID]bool{},
		Error:  nil,
	}
}

func (tf *TickerFactory) ListAccessible() []connection.ConnID {
	tf.Lock.RLock()
	defer tf.Lock.RUnlock()
	accessible := make([]connection.ConnID, 0, len(tf.Active))
	for conn := range tf.Active {
		accessible = append(accessible, conn)
	}
	return accessible
}

func (tf *TickerFactory) NewWithProps(p connection.ConnectionProps) (connection.RawConnection, error) {
	tf.Lock.Lock()
	defer tf.Lock.Unlock()

	if tf.Error != nil {
		return connection.RawConnection{}, tf.Error
	}
	reader, writer := make(chan []byte), make(chan []byte)
	errChan := make(chan error)
	connCtx, connCtxCancel := context.WithCancel(tf.Ctx)

	// producer subroutine
	// will produce data in intervals
	go func() {
		// these resources should be
		// released in this goroutine
		// as this is the one sending to channel
		ticker := time.NewTicker(tf.Period)
		defer func() {
			close(reader)
			ticker.Stop()
		}()

		for {
			select {
			case <-connCtx.Done():
				log.Println(p.ID(), "producer context done")
				return
			case <-ticker.C:
				log.Println(p.ID(), "Ticked")
				reader <- []byte("Ticked!")
			}
		}
	}()

	// consumer subroutine
	// will consume incoming messages and
	// print them out
	go func() {
		for {
			select {
			case <-connCtx.Done():
				log.Println(p.ID(), "reciever context done")
				return
			case chunk := <-writer:
				log.Println(p.ID(), "recieved data:", string(chunk))
			}
		}
	}()

	// attach hook to the connection instance
	return connection.RawConnection{
		ReaderChan: reader,
		WriterChan: writer,
		ErrChan:    errChan,
		CloseHook: func() error {
			select {
			case <-connCtx.Done():
				return connection.ErrAlreadyClosed
			default:
				connCtxCancel()
				close(writer)
				close(errChan)
				return nil
			}
		},
	}, nil
}
