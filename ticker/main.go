package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

//wss://stream.binance.com:9443/ws/bnbusdt@depth20@100ms
// wss://openws.58ex.com/v1/stream
// streams=<streamName1>/<streamName2>/<streamName3>
// wss://www.bitmex.com/realtime?subscribe=instrument,orderBook:XBTUSD
// 满币 wss://ws.coinbene.vip/stream/ws
// {"op":"subscribe","args":["usdt/orderBook.BTC-SWAP.10","usdt/tradeList.BTC-SWAP"]}
var addr = flag.String("addr", "stream.binance.com:9443", "http service address")

type CommandStruct struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/ws/btcusdt@depth20@100ms"}
	//u := url.URL{Scheme: "wss", Host: *addr, Path: "/realtime?subscribe=instrument,orderBook:XBTUSD"}
	log.Printf("connecting to %s", u.String())
	dialer := websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
	}

	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dail: ", err)
	}
	defer c.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if err != nil {
				fmt.Println("write json message: %w", err)
			}
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv:%s", message)
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}
