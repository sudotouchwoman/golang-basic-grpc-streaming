package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/golang-basic-grpc-streaming/pkg/web"
)

func main() {
	// sample websocket client (used as a demo and for debugging)
	// this one performs a single action: discovers connections
	// or reads from a one
	method := web.MethodRead
	flag.StringVar(&method, "action", method, "Read/close/discover")
	flag.Parse()

	done := make(chan bool)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	socketUrl := "ws://localhost:8080/ws"
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go func() {
		defer func() {
			done <- true
			close(done)
			close(interrupt)
		}()

		for {
			// read all messages from server
			// note that read timeout is updated on each iteration too,
			// just like on server side
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			select {
			case <-interrupt:
				return
			default:
				// in this demo, json response is simply stored in map
				response := map[string]interface{}{}
				_, msg, err := conn.ReadMessage()
				if err != nil {
					log.Println("err in reader:", err)
					return
				}
				log.Println(string(msg))
				// (actually unmarshalling is only required to
				// understand when it is time to disconnect
				// (this demo client only listens for a single stream)
				err = json.Unmarshal(msg, &response)
				if err != nil {
					log.Println("err in unmarshal:", err)
					return
				}
				// if err, ok := response["error"]; ok {
				// 	log.Println("server:", err)
				// 	return
				// }
			}
		}
	}()

	request := web.SerialRequest{
		Method:  method,
		Timeout: int64(10 * time.Second),
		BasicSerialMessage: web.BasicSerialMessage{
			Serial: "/dev/ttyUSB0",
		},
	}

	// as discussed in the comments above,
	// perform a single action (here, read stream of data)
	if err = conn.WriteJSON(&request); err != nil {
		log.Fatal("ws write:", err)
	}

	// also connect to another producer
	// data should be sent from both
	request.Serial = "/dev/ttyUSB2"
	if err = conn.WriteJSON(&request); err != nil {
		log.Fatal("ws write:", err)
	}
	// ask server to stop listening for /dev/ttyUSB0 after 10s
	// client still won't stop because it is still listening for /dev/ttyUSB2
	<-time.After(10 * time.Second)
	if err = conn.WriteJSON(&web.SerialRequest{
		Method: web.MethodStop,
		BasicSerialMessage: web.BasicSerialMessage{
			Serial: "/dev/ttyUSB0",
		},
	}); err != nil {
		log.Fatal("ws write:", err)
	}
	// do not exit until listener goroutine exits
	<-done
}
