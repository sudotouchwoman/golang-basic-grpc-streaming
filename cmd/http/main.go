package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/server"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/web"
)

func main() {
	// minimalistic http server to handle ws traffic
	// and stream from opened connections to clients
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// provider - gRPC client implementation
	// (for now, a mock factory is used)
	tickerFactory := server.NewTickerFactory(ctx, 2*time.Second)
	for _, dev := range []connection.ConnID{"/dev/ttyUSB0", "COM5", "COM7"} {
		tickerFactory.Active[dev] = true
	}
	provider := connection.NewConnectionProvider(
		ctx, tickerFactory,
	)
	factory := web.LogSockClientFactory{
		Ctx: ctx, Provider: provider,
		ReadTimeout: time.Minute,
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	wsHandler := web.SocketHandler(&factory, &upgrader)
	router := mux.NewRouter()
	router.Handle("/ws", wsHandler)
	router.Use(PanicRecovery, LogRemoteAddr)
	// serve static content (index and js bundle)
	router.PathPrefix("/assets").Handler(http.FileServer(http.Dir("./dist/")))
	router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "dist/index.html")
	})

	log.Println("starting ws server")
	log.Fatal(http.ListenAndServe("localhost:8080", router))
}
