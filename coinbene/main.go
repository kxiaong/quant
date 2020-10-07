package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/go-pg/pg"
	"github.com/gorilla/websocket"
	"github.com/kxiaong/quant/coinbene/model"
	"github.com/kxiaong/quant/util"
)

//"usdt/orderBook.BTC-SWAP",
// []string{"usdt/kline.BTC-SWAP", "usdt/ticker.BTC-SWAP", "usdt/orderBook.BTC-SWAP", "usdt/tradeList.BTC-SWAP"},
// []string{"usdt/kline.BTC-SWAP.1m" "usdt/kline.BTC-SWAP.5m" "usdt/kline.BTC-SWAP.15m" "usdt/kline.BTC-SWAP.30m" "usdt/kline.BTC-SWAP.1h"}
var (
	logger = util.GetLogger()
	dbconn = pg.Connect(&pg.Options{
		Addr:     "localhost:5432",
		User:     "coinbene",
		Password: "xiaoxiao",
		Database: "coinbene",
	})

	ApiKey    = "a13d876a61511b51c6dc5cb915ad7d02"
	ApiSecret = "3519d8e29cdc4952933c1b0c2b7f3625"
)

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

type KlineElem struct {
	KOpen   float64 `json:"o"`
	KClose  float64 `json:"c"`
	KHigh   float64 `json:"h"`
	KLow    float64 `json:"l"`
	KVolume float64 `json:"v"`
	KTs     int64   `json:"t"`
}

type KlineResponse struct {
	Topic  string      `json:"topic"`
	Action string      `json:"action"`
	Data   []KlineElem `json:"data"`
}

type AssetInfo struct {
	Asset            string  `json:"asset"`
	AvailableBalance float64 `json:"availableBalance"`
	FronzenBalance   float64 `json:"frozenBalance"`
	Balance          float64 `json:"balance"`
	Timestamp        string  `json:"timestamp"`
}

type AccountInfo struct {
	Topic     string      `json:"topic"`
	AssetList []AssetInfo `json:"data"`
}

func (o *Order) UnmarshalJSON(d []byte) error {
	var s []string
	if err := json.Unmarshal(d, &s); err != nil {
		return err
	}

	if len(s) < 2 {
		return fmt.Errorf("data length < 2")
	}

	p, err := strconv.ParseFloat(s[0], 64)
	if err != nil {
		return err
	}
	o.Price = p
	a, err := strconv.ParseFloat(s[1], 64)
	if err != nil {
		return err
	}
	o.Amount = a
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

	var addr = flag.String("addr", "ws.coinbene.com", "websocket server address")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/stream/ws"}
	dailer := websocket.Dialer{Proxy: http.ProxyFromEnvironment}

	c, _, _ := dailer.Dial(u.String(), nil)
	defer c.Close()
	defer dbconn.Close()

	msg := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/kline.BTC-SWAP.1m"},
	}

	err := c.WriteJSON(msg)
	if err != nil {
		logger.Println("subscribe failed:", err)
		return
	}

	err = login(c)
	if err != nil {
		return
	}

	err = GetUserHolding(c)
	//err = GetUserAccount(c)
	if err != nil {
		logger.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			logger.Println(string(message))
			if err != nil {
				logger.Println(err)
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
	respBrief := ResponseBrief{}
	err := json.Unmarshal(message, &respBrief)
	if err != nil {
		logger.Println("unmarshal resp brief failed: ", err)
		return
	}

	switch respBrief.Topic {
	case "usdt/orderBook.BTC-SWAP":
		err := processOrderBook(message)
		if err != nil {
			logger.Fatal(err)
		}
	case "usdt/kline.BTC-SWAP.1m":
		err := processKline(message)
		if err != nil {
			logger.Fatal(err)
		}
	default:
		return
	}
}

func processKline(message []byte) error {
	klineResp := KlineResponse{}
	err := json.Unmarshal(message, &klineResp)
	if err != nil {
		return err
	}

	if klineResp.Action == "insert" {
		var insertData []*model.Kline
		for _, e := range klineResp.Data {
			tmp := &model.Kline{
				High:   e.KHigh,
				Low:    e.KLow,
				Open:   e.KOpen,
				Close:  e.KClose,
				Volume: e.KVolume,
				Ts:     time.Unix(e.KTs, 0),
			}
			tmp.CreatedAt = time.Now()
			tmp.UpdatedAt = time.Now()
			insertData = append(insertData, tmp)
		}

		for _, i := range insertData {
			_, err := dbconn.Model(i).Insert()
			if err != nil {
				logger.Fatal(err)
				return err
			}
		}
	} else {
		logger.Println(string(message))
		for _, e := range klineResp.Data {
			logger.Println(e.KTs)
			tmp := &model.Kline{
				High:   e.KHigh,
				Low:    e.KLow,
				Open:   e.KOpen,
				Close:  e.KClose,
				Volume: e.KVolume,
				Ts:     time.Unix(e.KTs, 0),
			}
			tmp.UpdatedAt = time.Now()

			result, err := dbconn.Model(tmp).
				Set("high = ?", tmp.High).
				Set("low = ?", tmp.Low).
				Set("open = ?", tmp.Open).
				Set("close = ?", tmp.Close).
				Set("Volume = ?", tmp.Volume).
				Where("ts = ?", tmp.Ts).Update()

			if err != nil {
				return err
			}

			logger.Println("result: ", result)
		}
	}

	return nil
}

func processOrderBook(message []byte) error {
	d := OrderBookResponse{}
	err := json.Unmarshal(message, &d)
	if err != nil {
		logger.Println("unmarshal order book failed: ", err)
		return err
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

func login(c *websocket.Conn) error {
	sessionExpireDuration, err := time.ParseDuration("30m")
	if err != nil {
		logger.Fatal(err)
		return err
	}
	expireAt := time.Now().Add(sessionExpireDuration)
	expireAtStr := expireAt.Format("2006-01-02T15:04:05Z")
	command, err := signForWebSocket(expireAtStr, "GET", "/login")
	if err != nil {
		logger.Fatal(err)
	}

	t, _ := json.Marshal(command)
	logger.Println(string(t))
	err = c.WriteJSON(command)
	fmt.Println(command)
	if err != nil {
		logger.Fatalf("login failed: %v", err)
		return err
	}
	return nil
}

func signForWebSocket(expire string, method string, path string) (*CommandStruct, error) {
	shaResource := expire + method + path
	logger.Println("sha raw string: ", shaResource)
	sign := hmacSha256(shaResource, ApiSecret)
	command := &CommandStruct{
		Op:   "login",
		Args: []string{ApiKey, expire, sign},
	}
	return command, nil
}

func hmacSha256(data string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func GetUserAccount(c *websocket.Conn) error {
	command := CommandStruct{
		Op:   "subscribe",
		Args: []string{"contract/user.account"},
	}

	err := c.WriteJSON(command)
	if err != nil {
		logger.Fatalf("Get user account failed: %v", err)
		return err
	}
	return nil
}

func GetUserHolding(c *websocket.Conn) error {
	command := CommandStruct{
		Op:   "subscribe",
		Args: []string{"usdt/user.position"},
	}
	err := c.WriteJSON(command)
	if err != nil {
		logger.Println(err)
	}
	return nil
}

func processUserAccount(message []byte) (*AccountInfo, error) {
	d := &AccountInfo{}
	err := json.Unmarshal(message, d)
	if err != nil {
		logger.Fatalf("unmarshal account info failed: %v", err)
		return nil, err
	}
	return d, nil
}
