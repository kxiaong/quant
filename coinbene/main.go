package main

import (
	"flag"
	"log"
	"net/http"
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

type DataResponse struct {
}

//wss://ws.coinbene.com/stream/ws
func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/stream/ws"}
	dailer := websocket.Dialer{Proxy: http.ProxyFromEnvironment}

	c, _, _ := dailer.Dial(u.String(), nil)
	defer c.Close()
	msg := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/kline.ETH-SWAP", "usdt/ticker.ETH-SWAP", "usdt/orderBook.ETH-SWAP.100", "usdt/tradeList.ETH-SWAP"},
	}
	c.WriteJSON(msg)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, _ := c.ReadMessage()

			log.Printf("recv: %s", message)
			if string(message) == "ping" {
				pong := []byte("pong")
				c.WriteMessage(websocket.TextMessage, pong)
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
