package serial

import (
	"bufio"
	"context"
	"log"
	"sync"

	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
	"go.bug.st/serial"
)

type SerialConnectionFactory struct {
	Mu  *sync.RWMutex
	Ctx context.Context
}

func (sf *SerialConnectionFactory) NewWithProps(props connection.ConnectionProps) (raw connection.RawConnection, err error) {
	p, ok := props.(*server.LogStreamProps)
	if !ok {
		err = connection.ErrIncompatibleProps
		return
	}

	sf.Mu.Lock()
	defer sf.Mu.Unlock()

	com, err := serial.Open(p.Emitter, &serial.Mode{
		// some properties are default but can be configured later
		BaudRate: int(p.Baudrate),
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{
			RTS: false,
			DTR: false,
		},
	})
	if err != nil {
		log.Println("serial.Open():", err)
		return
	}
	log.Println(p.Emitter, "opened serial connection")

	serialCtx, serialCtxCancel := context.WithCancel(sf.Ctx)
	reader, writer := make(chan []byte), make(chan []byte)
	errChan := make(chan error)

	// raw connection is returned to the caller
	raw = connection.RawConnection{
		ErrChan:    errChan,
		WriterChan: writer,
		ReaderChan: reader,
		CloseHook: func() error {
			select {
			case <-serialCtx.Done():
				return connection.ErrAlreadyClosed
			default:
				serialCtxCancel()
				close(writer)
				log.Println(p.Emitter, "closing serial connection")
				com.Close()
				log.Println(p.Emitter, "closed serial connection")
				return nil
			}
		},
	}

	// this entity handles com-port communication
	// by using channels RawConnection stores
	serialConn := SerialConnection{
		ReadWriteCloser: com,
		Ctx:             serialCtx,
		Props:           p,
		Tokenizer:       bufio.ScanLines,
		WriterChannel:   writer,
		ReaderChannel:   reader,
		ErrChannel:      errChan,
	}
	// start listening for incoming records
	go serialConn.Listen()
	return
}

// Query serial library for the list of availible ports
// and return these to the caller. If something goes wrong,
// an empty slice is returned.
func (sf *SerialConnectionFactory) ListAccessible() []connection.ConnID {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Println("ListAccessible:", err)
		return []connection.ConnID{}
	}
	ids := make([]connection.ConnID, 0, len(ports))
	for i, port := range ports {
		ids[i] = connection.ConnID(port)
	}
	return ids
}
