package connection

import (
	"context"
	"sync"
)

type Broadcaster struct {
	Ctx           context.Context
	Producer      <-chan []byte
	TargetFactory func() []chan<- []byte
}

// Propagates updates from single producer
// to possibly several consumers (specified
// via TargetFactory attribute)
func (b *Broadcaster) Broadcast() {
	for {
		select {
		case <-b.Ctx.Done():
			return
		case chunk := <-b.Producer:
			for _, target := range b.TargetFactory() {
				select {
				case <-b.Ctx.Done():
					return
				case target <- chunk:
					continue
				}
			}
		}
	}
}

// Represents actually opened connection
type connection struct {
	Ctx        context.Context
	Mu         *sync.RWMutex
	Barrier    *sync.WaitGroup
	Props      ConnectionProps
	SendChan   chan<- []byte
	Peers      []chan<- []byte
	CancelHook func() error
}

// Add listener channel to the slice
func (conn *connection) AddPeer(ch chan<- []byte) {
	conn.Mu.Lock()
	defer conn.Mu.Unlock()
	conn.Peers = append(conn.Peers, ch)
}

// Delete listener channel from the slice
func (conn *connection) RemovePeer(ch chan<- []byte) {
	conn.Mu.Lock()
	defer conn.Mu.Unlock()
	for i, peer := range conn.Peers {
		if peer == ch {
			conn.Peers = remove(conn.Peers, i)
			return
		}
	}
}

// Helper function to drop element from slice
func remove(s []chan<- []byte, i int) []chan<- []byte {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// Creates new proxy from this connection.
// Schedules cleanup hooks to run in separate goroutines.
func (conn *connection) NewProxy() (ConnectionProxy, error) {
	// increment the counter and create new context
	// from connection's one
	conn.Barrier.Add(1)
	proxyCtx, proxyCtxCancel := context.WithCancel(conn.Ctx)

	// create new connReciever channel
	// for this request
	connReciever := make(chan []byte)
	conn.AddPeer(connReciever)

	proxy := &ChannelConnectionProxy{
		Ctx:      proxyCtx,
		SendChan: conn.SendChan,
		RecvChan: connReciever,
	}

	proxyCancelLock := &sync.Mutex{}
	hook := func() error {
		// cancel context of this consumer
		// and decrement barrier
		// make sure to do this atomically
		proxyCancelLock.Lock()
		defer proxyCancelLock.Unlock()
		if proxy.Done {
			return ErrAlreadyClosed
		}
		// we should also remove the peer from
		// the list of peers
		conn.RemovePeer(connReciever)
		proxy.Done = true
		proxyCtxCancel()
		conn.Barrier.Done()
		return nil
	}
	proxy.CancelHook = hook

	// once parent context is done,
	// underlying connections should be closed too
	go func() {
		<-conn.Ctx.Done()
		// close silently
		_ = hook()
	}()
	return proxy, nil
}

// Provider implementation for connection management
type ConnectionProvider struct {
	connFactory ConnectionFactory
	ctx         context.Context
	mu          *sync.RWMutex
	connections map[ConnID]*connection
}

// Creates new proxy to connection with given props.
// If the connection already exists, spawn a proxy from it.
// Otherwise, try to create a new connection first and spawn the proxy
// afterwards.
func (pr *ConnectionProvider) Open(props ConnectionProps) (ConnectionProxy, error) {
	// sync of this process might be performed in a more optimal way
	// yet for now lock the provider globally to avoid races
	pr.mu.Lock()
	defer pr.mu.Unlock()
	id := props.ID()
	if conn, exists := pr.connections[id]; exists {
		// it might be a good idea to compare other properties
		// before new proxy creation
		if !conn.Props.Compatible(props) {
			return nil, ErrIncompatibleProps
		}
		return conn.NewProxy()
	}

	// such connection does not exist yet thus must be created
	// get channels from somewhere
	rawConn := pr.connFactory.NewWithProps(props)
	if rawConn.Err != nil {
		return nil, rawConn.Err
	}

	connCtx, connCtxCancel := context.WithCancel(pr.ctx)
	group := &sync.WaitGroup{}

	conn := &connection{
		Ctx:      connCtx,
		Mu:       &sync.RWMutex{},
		Barrier:  group,
		Props:    props,
		SendChan: rawConn.WriterChan,
		Peers:    []chan<- []byte{},
	}

	// hook needs a closure for the connection
	// in order to safely check the peer count
	// thus it is attached separately
	hook := func() error {
		// conn.Mu.RLock()
		// defer conn.Mu.RUnlock()
		// if len(conn.Peers) == 0 {
		// 	return ErrAlreadyClosed
		// }
		connCtxCancel()
		pr.mu.Lock()
		defer pr.mu.Unlock()
		delete(pr.connections, id)
		return rawConn.CloseHook()
	}
	conn.CancelHook = hook

	broadcaster := &Broadcaster{
		Ctx:      connCtx,
		Producer: rawConn.ReaderChan,
		TargetFactory: func() []chan<- []byte {
			conn.Mu.RLock()
			defer conn.Mu.RUnlock()
			return conn.Peers
		},
	}
	// start redirecting messages
	// from this connection to whoever is listening
	// for them
	go broadcaster.Broadcast()

	// register in the map so that the subsequent
	// calls would only create new proxies to this connection
	pr.connections[id] = conn

	// wait until all consumers cancel their requests
	// and shut the connection down
	go func(wg *sync.WaitGroup, id ConnID) {
		wg.Wait()
		_ = conn.CancelHook()
	}(group, id)

	return conn.NewProxy()
}
