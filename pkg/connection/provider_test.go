package connection

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"
)

type DummyFactory struct {
	// factory context (parent for all child connection contexts)
	Ctx context.Context
	// channel polled with a period of Period
	Producer chan []byte
	// delay between subsequent emissions
	Period time.Duration
	Active []ConnID
	// when non-nil, calls to NewWithProps would fail
	Err error
}

func (df *DummyFactory) NewWithProps(p ConnectionProps) (RawConnection, error) {
	if df.Err != nil {
		return RawConnection{}, df.Err
	}
	reader, writer := make(chan []byte), make(chan []byte)
	connCtx, connCtxCancel := context.WithCancel(df.Ctx)
	errChan := make(chan error)

	// start emitting dummy messages
	go func() {
		for p := range df.Producer {
			select {
			case <-connCtx.Done():
				log.Println("producer stopped")
				return
			default:
				msg := string(p)
				log.Println("dummy will send:", msg)
				reader <- p
				log.Println("dummy sent:", msg)
			}
			time.Sleep(df.Period)
		}
	}()

	// start recieving dummy messages
	go func() {
		for {
			select {
			case <-connCtx.Done():
				return
			case chunk := <-writer:
				log.Println("dummy recieve: ", string(chunk))
				select {
				case <-connCtx.Done():
					log.Println("reciever stopped")
				default:
					// echo the chunk back
					reader <- chunk
					log.Println("echoed: ", string(chunk))
				}
			}
		}
	}()

	return RawConnection{
		ReaderChan: reader,
		WriterChan: writer,
		ErrChan:    errChan,
		CloseHook: func() error {
			connCtxCancel()
			close(reader)
			close(writer)
			close(errChan)
			return nil
		},
	}, nil
}

func (df *DummyFactory) ListAccessible() []ConnID {
	return df.Active
}

type DummyProps struct {
	id       ConnID
	Baudrate int
}

func (dp *DummyProps) ID() ConnID {
	return dp.id
}

func (dp *DummyProps) Compatible(other ConnectionProps) bool {
	switch other := other.(type) {
	case *DummyProps:
		return dp.id == other.id && dp.Baudrate == other.Baudrate
	default:
		return false
	}
}

func TestDummyFactory_SeveralClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// sample data source for two clients
	// expected records is a sample collection of words
	producer := make(chan []byte, 10)
	expectedRecords := []string{"1", "2", "3", "4", "5", "6"}
	for _, record := range expectedRecords {
		producer <- []byte(record)
	}

	factory := DummyFactory{
		Ctx:      ctx,
		Producer: producer,
		Period:   time.Second,
		Active:   []ConnID{"0"},
		Err:      nil,
	}
	provider := NewConnctionProvider(ctx, &factory)
	tt := &DummyProps{"0", 200}

	proxyOne, err := provider.Open(tt)
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}
	proxyTwo, err := provider.Open(tt)
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}

	// in this example, close first client
	// after 5s and the second one after 10s
	// the first client should be unable to send/recieve after 5s
	go func(t *testing.T) {
		time.Sleep(1 * time.Second)
		log.Println("want to stop proxyOne")
		if err := proxyOne.Close(); err != nil {
			t.Errorf("proxyOne.Close() error=%v\n", err)
			return
		}
		log.Println("stopped proxyOne")
	}(t)

	go func(t *testing.T) {
		time.Sleep(6 * time.Second)
		log.Println("want to stop proxyTwo")
		if err := proxyTwo.Close(); err != nil {
			t.Errorf("proxyTwo.Close() error=%v\n", err)
			return
		}
		log.Println("stopped proxyTwo")
	}(t)

	// loop over incoming requests
	// they should be broadcasted to both proxies
	// client 1
	go PongMessages(t, proxyOne, 2*time.Second, func(b []byte) {})
	// go func(t *testing.T) {
	// 	for {
	// 		from := time.Now()
	// 		log.Println("1 waiting for next chunk")
	// 		if _, errRecv := proxyOne.Recv(2 * time.Second); errRecv != nil {
	// 			if errRecv == io.EOF {
	// 				log.Println("1 stream end")
	// 				break
	// 			}
	// 			t.Errorf("1 proxy.Recv() error=%v (eta %s)\n", errRecv, time.Since(from))
	// 			break
	// 		}
	// 	}
	// }(t)

	clientTwoLog := make([]string, 0)
	PongMessages(t, proxyTwo, 2*time.Second, func(b []byte) {
		clientTwoLog = append(clientTwoLog, string(b))
	})
	// client 2
	// for {
	// 	from := time.Now()
	// 	log.Println("2 waiting for next chunk")
	// 	if msg, errRecv := proxyTwo.Recv(2 * time.Second); errRecv != nil {
	// 		if errRecv == io.EOF {
	// 			log.Println("2 stream end")
	// 			break
	// 		}
	// 		t.Errorf("2 proxy.Recv() error=%v (eta %s)\n", errRecv, time.Since(from))
	// 		break
	// 	} else {
	// 		// guess that
	// 		// remember this record (operation must be quick enough
	// 		// so that the broadcaster does not skip the redirect)
	// 		clientTwoLog = append(clientTwoLog, string(msg))
	// 	}
	// }
	for i, expected := range expectedRecords {
		record := clientTwoLog[i]
		if record != expected {
			t.Errorf("Integrity error: want %s, got %s", expected, record)
			t.Errorf("Expected: %v, Got: %v", expectedRecords, clientTwoLog)
		}
	}
}

func TestDummyFactory_NewWithProps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// some external source of data for the connections
	producer := make(chan []byte, 5)
	for i := 0; i < 5; i++ {
		msg := fmt.Sprint("i=", i)
		producer <- []byte(msg)
	}

	factory := DummyFactory{
		Ctx:      ctx,
		Producer: producer,
		Period:   5e2 * time.Millisecond,
		Active:   []ConnID{"0", "4", "14"},
		Err:      nil,
	}
	provider := NewConnctionProvider(ctx, &factory)
	tt := &DummyProps{"0", 115200}

	proxy, err := provider.Open(tt)
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}
	// close after some time
	go func(t *testing.T) {
		time.Sleep(10 * time.Second)
		log.Println("want to stop proxying")
		if err := proxy.Close(); err != nil {
			t.Errorf("proxy.Close() error=%v\n", err)
			return
		}
		log.Println("stopped proxy")
	}(t)

	messages := make(chan []byte, 10)

	// work with incoming requests
	go PingMessages(t, proxy, 500*time.Millisecond, time.Millisecond, messages)
	PongMessages(t, proxy, time.Second, func(b []byte) {
		messages <- b
	})
	close(messages)
	log.Println("closed chan messages")
}

func PingMessages(t *testing.T, proxy ConnectionProxy, delay, timeout time.Duration, messages <-chan []byte) {
	for msg := range messages {
		// wait for a bit
		time.Sleep(delay)
		log.Println("will echo back", string(msg))
		from := time.Now()
		if errSend := proxy.Send(msg, time.Millisecond); errSend != nil {
			if errSend == ErrAlreadyClosed {
				log.Println("send failed: channel closed")
				continue
			}
			t.Errorf("proxy.Send() error=%v\n", errSend)
		}
		log.Printf("sent %s: eta %s\n", msg, time.Since(from))
	}
	log.Println("all messages processed")
}

func PongMessages(t *testing.T, proxy ConnectionProxy, timeout time.Duration, onMessage func([]byte)) {
	// loop over incoming requests
	for {
		from := time.Now()
		log.Println("waiting for next chunk")
		if msg, errRecv := proxy.Recv(timeout); errRecv != nil {
			if errRecv == io.EOF {
				log.Println("stream end")
				break
			}
			t.Errorf("proxy.Recv() error=%v (eta %s)\n", errRecv, time.Since(from))
			break
		} else {
			// delegate info to consumer
			onMessage(msg)
		}
	}
}
