package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"

	"github.com/gorilla/websocket"
)

// []string{"usdt/kline.BTC-SWAP", "usdt/ticker.BTC-SWAP", "usdt/orderBook.BTC-SWAP", "usdt/tradeList.BTC-SWAP"},
// []string{"usdt/kline.BTC-SWAP.1m" "usdt/kline.BTC-SWAP.5m" "usdt/kline.BTC-SWAP.15m" "usdt/kline.BTC-SWAP.30m" "usdt/kline.BTC-SWAP.1h"}

var logger = GetLogger()

func GetLogger() *log.Logger {
	f, err := os.OpenFile("coinbene.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	logger := log.New(f, "coinbene: ", log.LstdFlags)
	return logger
}

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

	var addr = flag.String("addr", "ws.coinbene.com", "websocket server address")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/stream/ws"}
	dailer := websocket.Dialer{Proxy: http.ProxyFromEnvironment}

	c, _, _ := dailer.Dial(u.String(), nil)
	defer c.Close()
	msg := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/orderBook.BTC-SWAP"},
	}
	c.WriteJSON(msg)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				logger.Fatal(err)
				continue
			}

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
	logger.Printf("dispatch: %s", message)
	respBrief := ResponseBrief{}
	err := json.Unmarshal(message, &respBrief)
	if err != nil {
		logger.Fatal(err)
	}

	if respBrief.Topic == "usdt/orderBook.BTC-SWAP" {
		processOrderBook(message)
	} else if respBrief.Topic == "usdt/kline.BTC-SWAP.1m" {
		return
	} else {
		return
	}
}

func processOrderBook(message []byte) error {
	logger.Println("process OrderBook")
	d := OrderBookResponse{}
	err := json.Unmarshal(message, &d)
	if err != nil {
		logger.Fatal(err)
	}

	for _, e := range d.Data {
		logger.Println(e.Version)
		logger.Println(e.Timestamp)
		logger.Println("bids ===>")
		for _, b := range e.Bids {
			logger.Println(b)
		}
		logger.Println("asks ===>")
		for _, a := range e.Asks {
			logger.Println(a)
		}
	}
	return nil
}
