package web

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/connection"
)

type SockHandler interface {
	Read()
	Write()
}

// Creates and manages
type SockHandlerFactory interface {
	New(*websocket.Conn) SockHandler
}

// Message format for ws requests from clients
type SerialRequest struct {
	BasicSerialMessage
	Method   string `json:"method"`
	Baudrate uint32 `json:"baudrate,omitempty"`
	Timeout  int64  `json:"timeout"`
}

// Response format of ws server
type BasicSerialMessage struct {
	Message string    `json:"message"`
	Serial  string    `json:"serial"`
	Iat     time.Time `json:"iat"`
}

type DiscoverySerialMessage struct {
	Serials []connection.ConnID `json:"serials"`
	Iat     time.Time           `json:"iat"`
}

type ErrorSerialMessage struct {
	Serial string `json:"serial,omitempty"`
	Error  string `json:"error"`
}

func JsonifyError(err error, serial string) []byte {
	msg, err := json.Marshal(&ErrorSerialMessage{
		Serial: serial,
		Error:  err.Error(),
	})
	if err != nil {
		log.Fatalln("marshal:", err)
	}
	log.Println("jsonify:", string(msg))
	return msg
}
