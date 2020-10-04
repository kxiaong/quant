package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/kxiaong/quant/util"
)

// []string{"usdt/kline.BTC-SWAP", "usdt/ticker.BTC-SWAP", "usdt/orderBook.BTC-SWAP", "usdt/tradeList.BTC-SWAP"},
// []string{"usdt/kline.BTC-SWAP.1m" "usdt/kline.BTC-SWAP.5m" "usdt/kline.BTC-SWAP.15m" "usdt/kline.BTC-SWAP.30m" "usdt/kline.BTC-SWAP.1h"}

type CommandStruct struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

type ResponseBrief struct {
	Topic  string `json:"topic"`
	Action string `json:"action"`
}

type EventResponse struct {
	Event string `json:"event"`
	Topic string `json:"topic"`
	Code  int    `json:"code,omitempty"`
}

type Order struct {
	Price  float64
	Amount float64
}

func (o *Order) UnmarshalJSON(d []byte) error {
	var s []string
	if err := json.Unmarshal(d, &s); err != nil {
		return err
	}

	if len(s) >= 2 {
		if p, err := strconv.ParseFloat(s[0], 64); err != nil {
			return err
		} else {
			o.Price = p
		}

		if a, err := strconv.ParseFloat(s[1], 64); err != nil {
			return err
		} else {
			o.Amount = a
		}
	}
	return nil
}

type OrderBookData struct {
	Bids      []Order `json:"bids"`
	Asks      []Order `json:"asks"`
	Version   int64   `json:"version"`
	Timestamp uint64  `json:"timestamp"`
}

type OrderBookResponse struct {
	Topic  string          `json:"topic"`
	Action string          `json:"action"`
	Data   []OrderBookData `json:"data"`
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	logger := util.GetLogger()
	logger.Println("starting...")
	var addr = flag.String("addr", "ws.coinbene.com", "websocket server address")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	fmt.Println("begin......")

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/stream/ws"}
	dailer := websocket.Dialer{Proxy: http.ProxyFromEnvironment}

	c, _, _ := dailer.Dial(u.String(), nil)
	defer c.Close()
	msg := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/orderBook.BTC-SWAP"},
	}

	fmt.Println("subscribe...")
	err := c.WriteJSON(msg)
	if err != nil {
		fmt.Printf("subscribe failed: %w", err)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println(err)
				continue
			}

			fmt.Println("begion to process message...")
			if string(message) == "ping" {
				pong := []byte("pong")
				c.WriteMessage(websocket.TextMessage, pong)
			} else {
				dispatch(message)
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			logger.Println("Interrupt by user input")
			return
		}
	}
}

func dispatch(message []byte) {
	fmt.Println("begin to process order book...")
	respBrief := ResponseBrief{}
	err := json.Unmarshal(message, &respBrief)
	if err != nil {
		fmt.Printf("unmarshal resp brief failed: %w", err)
		return
	}

	if respBrief.Topic == "usdt/orderBook.BTC-SWAP" {
		fmt.Println("process Order book")
		processOrderBook(message)
	} else if respBrief.Topic == "usdt/kline.BTC-SWAP.1m" {
		fmt.Println("process kline")
		return
	} else {
		fmt.Println("no match, return")
		return
	}
}

func processOrderBook(message []byte) error {
	fmt.Println("in processing of order book...")
	d := OrderBookResponse{}
	err := json.Unmarshal(message, &d)
	if err != nil {
		fmt.Printf("unmarshal order book failed: %w", err)
		return err
	}

	for _, e := range d.Data {
		fmt.Println(e.Version)
		fmt.Println(e.Timestamp)
		fmt.Println("bids ===>")
		for _, b := range e.Bids {
			fmt.Println(b)
		}
		fmt.Println("asks ===>")
		for _, a := range e.Asks {
			fmt.Println(a)
		}
	}
	return nil
}
