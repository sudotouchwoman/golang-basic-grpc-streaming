package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
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
	if rand.Float64() > 0.5 {
		return []connection.ConnID{}
	}
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
	// intermediate channel to communicate
	// between reader and writer goroutines
	// (essentially to redirect data from reader to writer)
	pipe := make(chan []byte)
	errChan := make(chan error)
	connCtx, connCtxCancel := context.WithCancel(tf.Ctx)

	// producer subroutine
	// will produce data in intervals
	go func() {
		// these resources should be
		// released in this goroutine
		// as this is the one sending to channel
		ticker := time.NewTicker(tf.Period)
		timer := time.NewTimer(tf.Period * 20)
		ticks := 0
		defer func() {
			close(reader)
			ticker.Stop()
			timer.Stop()
		}()

		// perform several ticks and close
		// the connection afterwards
		for {
			select {
			case <-connCtx.Done():
				log.Println(p.ID(), "producer context done")
				return
			case <-ticker.C:
				ticks++
				log.Println(p.ID(), "Ping")
				reader <- []byte(fmt.Sprint("Ping ", ticks))
			case <-timer.C:
				log.Println("emulates disconnect")
				errChan <- io.EOF
				continue
			case chunk := <-pipe:
				reader <- chunk
			}
		}
	}()

	// consumer subroutine
	// will consume incoming messages and
	// print them out
	go func() {
		defer close(pipe)
		for {
			select {
			case <-connCtx.Done():
				log.Println(p.ID(), "reciever context done")
				return
			case chunk := <-writer:
				log.Println(p.ID(), "recieved data:", string(chunk))
				pipe <- chunk
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
