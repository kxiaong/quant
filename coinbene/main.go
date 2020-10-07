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
	"strings"
	"time"

	"github.com/go-pg/pg"
	"github.com/go-redis/redis"
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

	rclient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
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

type TradeElem struct {
	Price     float64
	Direction string
	Amount    int64
	Ts        time.Time
}

func (t *TradeElem) UnmarshalJSON(d []byte) error {
	data := string(d)
	data = data[1 : len(data)-1]
	raw := strings.Split(data, ",")
	if len(raw) < 4 {
		return fmt.Errorf("trade element length < 4")
	}

	var p float64
	p, err := strconv.ParseFloat(strings.Trim(raw[0], "\""), 64)
	if err != nil {
		return err
	}
	t.Price = p

	_direction := strings.Trim(raw[1], "\"")
	if _direction == "b" {
		t.Direction = "buy"
	} else if _direction == "s" {
		t.Direction = "sell"
	} else {
		return fmt.Errorf("unknown Direction: %s", _direction)
	}

	var a int64
	if a, err = strconv.ParseInt(strings.Trim(raw[2], "\""), 10, 64); err != nil {
		return err
	}
	t.Amount = a

	var ts int64
	if ts, err = strconv.ParseInt(raw[3], 10, 64); err != nil {
		return err
	}
	t.Ts = time.Unix(0, ts*int64(time.Millisecond))
	return nil
}

type TradeListResponse struct {
	Topic  string      `json:"topic"`
	Action string      `json:"action"`
	Data   []TradeElem `json:"data"`
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

type Order struct {
	Price  float64
	Amount float64
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
		Op: "subscribe",
		Args: []string{
			//"usdt/kline.BTC-SWAP.1h",
			"usdt/kline.BTC-SWAP.1m",
			"usdt/ticker.BTC-SWAP",
			"usdt/orderBook.BTC-SWAP.100",
			"usdt/tradeList.BTC-SWAP",
		},
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

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
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
		if err := processOrderBook(message); err != nil {
			logger.Fatal(err)
		}
	case "usdt/kline.BTC-SWAP.1m":
		if err := processKline(message, "1m"); err != nil {
			logger.Fatal(err)
		}
	case "usdt/tradeList.BTC-SWAP":
		if err := processTradeList(message); err != nil {
			logger.Fatal(err)
		}
	default:
		return
	}
}

func processTradeList(message []byte) error {
	logger.Println(string(message))
	tradeList := TradeListResponse{}
	if err := json.Unmarshal(message, &tradeList); err != nil {
		return err
	}

	var tlist []*model.TradeList
	for _, e := range tradeList.Data {
		tmp := &model.TradeList{
			Price:     e.Price,
			Direction: e.Direction,
			Amount:    e.Amount,
			Ts:        e.Ts,
		}
		tlist = append(tlist, tmp)
	}

	for _, t := range tlist {
		if err := InsertTradeList(t); err != nil {
			logger.Fatal(err)
		}
	}

	return nil
}

func InsertTradeList(trade *model.TradeList) error {
	trade.CreatedAt = time.Now()
	trade.UpdatedAt = time.Now()
	if _, err := dbconn.Model(trade).Insert(); err != nil {
		return err
	}
	return nil
}

func processTicker(message []byte) error {
	logger.Println(string(message))
	return nil
}

func processOrderBook(message []byte) error {
	d := OrderBookResponse{}
	err := json.Unmarshal(message, &d)
	if err != nil {
		logger.Println("unmarshal order book failed: ", err)
		return err
	}

	if d.Action == "insert" {
		BuildOrderBook(&d)
	} else {
		err := UpdateOrderBook(&d)
		if err != nil {
			logger.Fatal(err)
		}
	}

	return nil
}

func UpdateOrderBook(orderBook *OrderBookResponse) error {
	for _, orderVersion := range orderBook.Data {
		version := orderVersion.Version
		askData := orderVersion.Asks
		bidData := orderVersion.Bids
		for _, ask := range askData {
			key := fmt.Sprintf("coinbene:btc:sell:%f", ask.Price)
			_, err := rclient.Get(key).Result()
			if err == redis.Nil {
				if ask.Amount != 0 {
					if _, err := rclient.Set(key, ask.Amount, time.Duration(10*5*time.Second)).Result(); err != nil {
						return err
					}
				} else {
					continue
				}
			} else if err != nil {
				return err
			}
			if ask.Amount != 0 {
				rclient.Set(key, ask.Amount, time.Duration(10*5*time.Second)).Result()
			}
		}

		for _, bid := range bidData {
			key := fmt.Sprintf("coinbene:btc:buy:%f", bid.Price)
			_, err := rclient.Get(key).Result()
			if err == redis.Nil {
				if bid.Amount == 0 {
					continue
				}
				if _, err := rclient.Set(key, bid.Amount, time.Duration(10*5*time.Second)).Result(); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			if bid.Amount == 0 {
				continue
			}
			rclient.Set(key, bid.Amount, time.Duration(10*5*time.Second)).Result()
		}
		rclient.Set("coinbene:btc:latest:version", version, time.Duration(10*5*time.Second))
	}
	return nil
}

func BuildOrderBook(orderBook *OrderBookResponse) error {
	// pattern coinbene:<currnecy>:<side>:<price>
	oldKeys, err := rclient.Keys("coinbene:btc:*:*").Result()
	if err != nil {
		return err
	}

	for _, k := range oldKeys {
		rclient.Del(k)
	}
	rclient.Del("coinbene:btc:latest:version")

	for _, orderVersion := range orderBook.Data {
		version := orderVersion.Version
		askData := orderVersion.Asks
		bidData := orderVersion.Bids
		for _, ask := range askData {
			if ask.Amount == 0 {
				continue
			}
			key := fmt.Sprintf("coinbene:btc:sell:%f", ask.Price)
			if _, err := rclient.Set(key, ask.Amount, time.Duration(10*60*time.Second)).Result(); err != nil {
				return err
			}
		}

		for _, bid := range bidData {
			if bid.Amount == 0 {
				continue
			}
			key := fmt.Sprintf("coinbene:btc:buy:%f", bid.Price)
			if _, err := rclient.Set(key, bid.Amount, time.Duration(10*60*time.Second)).Result(); err != nil {
				return err
			}
		}
		if _, err := rclient.Set("coinbene:btc:latest:version", version, time.Duration(10*60*time.Second)).Result(); err != nil {
			return err
		}
	}

	return nil
}

func processKline(message []byte, gap string) error {
	klineResp := KlineResponse{}
	err := json.Unmarshal(message, &klineResp)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	if klineResp.Action == "insert" {
		latest, err := GetLatestKlineTs(gap)
		if err != nil {
			return err
		}

		var insertData []*model.Kline
		for _, e := range klineResp.Data {
			if e.KTs <= latest.Unix() {
				continue
			}
			tmp := &model.Kline{
				High:   e.KHigh,
				Low:    e.KLow,
				Open:   e.KOpen,
				Close:  e.KClose,
				Volume: e.KVolume,
				Gap:    gap,
				Ts:     time.Unix(e.KTs, 0),
			}
			insertData = append(insertData, tmp)
		}

		for _, i := range insertData {
			err := InsertKline(i)
			if err != nil {
				return err
			}
		}
	} else {
		for _, e := range klineResp.Data {
			tmp := &model.Kline{
				High:   e.KHigh,
				Low:    e.KLow,
				Open:   e.KOpen,
				Close:  e.KClose,
				Volume: e.KVolume,
				Gap:    gap,
				Ts:     time.Unix(e.KTs, 0),
			}

			affected, err := UpdateKline(tmp)
			if err != nil {
				return err
			}
			if affected == 0 {
				return InsertKline(tmp)
			}
		}
	}
	return nil
}

func GetLatestKlineTs(gap string) (*time.Time, error) {
	var p time.Time
	err := dbconn.Model(&model.Kline{}).
		Column("ts").
		Where("gap = ?", gap).
		Order("ts desc").Limit(1).Select(&p)
	return &p, err
}

func InsertKline(kline *model.Kline) error {
	kline.CreatedAt = time.Now()
	kline.UpdatedAt = time.Now()
	_, err := dbconn.Model(kline).Insert()
	if err != nil {
		logger.Fatal(err)
		return err
	}
	return nil
}

func UpdateKline(kline *model.Kline) (int, error) {
	kline.UpdatedAt = time.Now()
	result, err := dbconn.Model(&model.Kline{}).
		Set("high = ?", kline.High).
		Set("low = ?", kline.Low).
		Set("close = ?", kline.Close).
		Set("volume = ?", kline.Volume).
		Set("updated_at = ?", kline.UpdatedAt).
		Where("ts = ?", kline.Ts).
		Where("gap = ?", kline.Gap).
		Update()

	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
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

func processUserAccount(message []byte) (*AccountInfo, error) {
	d := &AccountInfo{}
	err := json.Unmarshal(message, d)
	if err != nil {
		logger.Fatalf("unmarshal account info failed: %v", err)
		return nil, err
	}
	return d, nil
}
