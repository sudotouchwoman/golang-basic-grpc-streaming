package serial

import (
	"bufio"
	"context"
	"io"
	"log"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
)

type SerialConnection struct {
	// Provides some higher-order API to given io.ReadWriterCloser.
	// Basically, populates channels with data and
	// errors collected during scanning
	// Tokenizer attribute can be set to an appropriate function
	// to be used during scanning to tokenize the bytes.
	// bufio.ScanLines is set by default by ConnectionManager
	//  but this property is public thus can be modified by
	// the caller later (please note that modifying makes
	// no sense once the connection starts scanning)
	io.ReadWriteCloser
	Props         *server.LogStreamProps
	Tokenizer     bufio.SplitFunc
	WriterChannel <-chan []byte
	ReaderChannel chan<- []byte
	ErrChannel    chan<- error
	Ctx           context.Context
}

func (ss *SerialConnection) Listen() {
	// Starts scanning provided connection for tokens
	// This method is intended to be run in a separate goroutine
	id := ss.Props.ID()

	// reader subroutine
	// will scan the port and redirect updates to the channel
	go func() {
		scanner := bufio.NewScanner(ss.ReadWriteCloser)
		scanner.Split(ss.Tokenizer)
		// cleanup resources
		// once done
		defer func() {
			close(ss.ReaderChannel)
			close(ss.ErrChannel)
		}()
		// read from the port and redirect contents
		// into the channel
		for scanner.Scan() {
			log.Println(id, scanner.Text())
			select {
			case <-ss.Ctx.Done():
				log.Println(id, "producer context done")
				ss.ErrChannel <- io.EOF
				return
			case ss.ReaderChannel <- scanner.Bytes():
			}
		}
		log.Println("scan done")
		// once done, we should check on the context
		// to decide whether it was port fault or it was closed
		// on purpose
		select {
		case <-ss.Ctx.Done():
			log.Println(id, "producer context done")
			ss.ErrChannel <- io.EOF
			return
		default:
			log.Println(id, "connection was interrupted before context finished")
			ss.ErrChannel <- scanner.Err()
		}
	}()

	// writer subroutine
	// will redirect all incoming writes to the port
	go func() {
		for {
			select {
			case <-ss.Ctx.Done():
				log.Println(id, "reciever context done")
				return
			case chunk, open := <-ss.WriterChannel:
				if !open {
					return
				}
				log.Println(id, string(chunk))
				if _, err := ss.ReadWriteCloser.Write(chunk); err != nil {
					log.Println(id, "write err:", err)
				}
			}
		}
	}()
}
