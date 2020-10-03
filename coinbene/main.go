package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "ws.coinbene.com", "websocket server address")

type CommandStruct struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

//wss://ws.coinbene.com/stream/ws
func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/stream/ws"}
	log.Printf("connecting to %s", u.String())
	dailer := websocket.Dialer{}

	c, _, err := dailer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial: ", err)
		return
	}
	defer c.Close()
	msg := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/ticker.BTC-SWAP", "usdt/orderBook.BTC-SWAP.10", "usdt/tradeList.BTC-SWAP"},
	}
	err = c.WriteJSON(msg)
	if err != nil {
		log.Printf("write json: %v", err)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			log.Println("waiting for new message...")
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			if string(message) == "ping" {
				pong := []byte("pong")
				c.WriteMessage(len(pong), pong)
				log.Println("pong")
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("Interrupt by user input")
			return
		}
	}

}
