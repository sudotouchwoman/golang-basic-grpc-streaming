package web

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Http handler for ws endpoint
func SocketHandler(factory SockHandlerFactory, upgrader *websocket.Upgrader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade error:", err)
			return
		}
		if handler := factory.New(ws); handler != nil {
			go handler.Write()
			handler.Read()
		}
	})
}
