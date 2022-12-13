package connection

import (
	"context"
	"fmt"
	"io"
	"log"
	"reflect"
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
				// no idea how to setup this test case so that
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
	// after 2s and the second one after 6s
	// the first client should be unable to send/recieve after 2s
	time.AfterFunc(9e2*time.Millisecond, func() {
		StopProxy(t, proxyOne)
	})
	time.AfterFunc(6*time.Second, func() {
		StopProxy(t, proxyTwo)
	})

	// loop over incoming requests
	// they should be broadcasted to both proxies
	// client 1
	clientOneLog := make([]string, 0)
	go PongMessages(t, proxyOne, 2*time.Second, func(b []byte) {
		clientOneLog = append(clientOneLog, string(b))
	})

	clientTwoLog := make([]string, 0)
	PongMessages(t, proxyTwo, 2*time.Second, func(b []byte) {
		clientTwoLog = append(clientTwoLog, string(b))
	})
	if !reflect.DeepEqual(expectedRecords, clientTwoLog) {
		t.Errorf("Expected: %v, Got: %v", expectedRecords, clientTwoLog)
	}
	if !reflect.DeepEqual(clientOneLog, expectedRecords[:1]) {
		t.Errorf(
			"Client one got unexpected items: %v, expected %v",
			clientOneLog, expectedRecords[:1],
		)
	}
}

func StopProxy(t *testing.T, proxy ConnectionProxy) {
	log.Println("want to stop proxy")
	if err := proxy.Close(); err != nil {
		t.Errorf("proxy.Close() error=%v\n", err)
		return
	}
	log.Println("stopped proxy")
	if err := proxy.Close(); err != ErrAlreadyClosed {
		t.Errorf("proxy.Close() no error on second call")
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
		Period:   2e2 * time.Millisecond,
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
	time.AfterFunc(10*time.Second, func() {
		StopProxy(t, proxy)
	})

	messages := make(chan []byte, 10)

	// work with incoming requests
	go PingMessages(t, proxy, time.Second, time.Millisecond, messages)
	PongMessages(t, proxy, 2*time.Second, func(b []byte) {
		messages <- b
	})
	close(messages)
	log.Println("closed chan messages")
}

func PingMessages(
	t *testing.T, proxy ConnectionProxy,
	delay, timeout time.Duration,
	messages <-chan []byte) {
	for msg := range messages {
		// wait for a bit
		time.Sleep(delay)
		log.Println("will echo back", string(msg))
		from := time.Now()
		if errSend := proxy.Send(msg, time.Millisecond); errSend != nil {
			if errSend == ErrAlreadyClosed {
				log.Println("send failed: channel closed")
				break
			}
			t.Errorf("proxy.Send() error=%v\n", errSend)
		}
		log.Printf("sent %s: eta %s\n", msg, time.Since(from))
	}
	log.Println("all messages processed")
}

func PongMessages(
	t *testing.T, proxy ConnectionProxy,
	timeout time.Duration, onMessage func([]byte)) {
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

func TestDummyFactory_ClientWithTimeout(t *testing.T) {
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

	proxy, err := provider.Open(tt)
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}

	time.AfterFunc(10*time.Second, func() {
		StopProxy(t, proxy)
	})
	// second call must time out since messages are
	// emitted approximitely once a second
	_, errRecv := proxy.Recv(2e2 * time.Millisecond)
	if errRecv != nil {
		t.Errorf("Expected no timeout, got=%s", errRecv)
		return
	}
	_, errRecv = proxy.Recv(2e2 * time.Millisecond)
	if errRecv != ErrTimedOut {
		t.Errorf("Expected timeout, got=%e", errRecv)
		return
	}

	// try opening connection with different parameters
	if _, err = provider.Open(&DummyProps{"0", 300}); err != ErrIncompatibleProps {
		t.Errorf("Expected incompatability, got=%e", err)
		return
	}

	// after canceling the main provider context
	// it is still possible to open connections,
	// however these are closed instantly
	cancel()
	_, err = provider.Open(tt)
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}
	_, errRecv = proxy.Recv(2e2 * time.Millisecond)
	if errRecv != io.EOF {
		t.Errorf("Expected EOF, got=%e", errRecv)
		return
	}
}

func TestDummyFactory_ProducerConnectionLists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedActive := []ConnID{"0"}
	expectedAccessible := []ConnID{"0", "1"}
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
		Active:   expectedAccessible,
		Err:      nil,
	}

	provider := NewConnctionProvider(ctx, &factory)
	// note that this proxy will be
	_, err := provider.Open(&DummyProps{"0", 100})
	if err != nil {
		t.Errorf("provider.Open() error=%v\n", err)
		return
	}
	// check on accessible/active connections
	active := provider.ListActive()
	if !reflect.DeepEqual(active, expectedActive) {
		t.Errorf(
			"Expected these active ports = %v, got= %v",
			active, expectedActive,
		)
	}
	accessible := provider.ListAccessible()
	if !reflect.DeepEqual(accessible, expectedAccessible) {
		t.Errorf(
			"Expected these active ports = %v, got= %v",
			accessible, expectedAccessible,
		)
	}
}
