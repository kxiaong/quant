package model

import "time"

type TradeList struct {
	tableName struct{}  `sql:"coinbene_trade_list"`
	CreatedAt time.Time `sql:"created_at"`
	UpdatedAt time.Time `sql:"updated_at"`
	Price     float64   `sql:"price"`
	Direction string    `sql:"direction"`
	Amount    int64     `sql:"amount"`
	Ts        time.Time `sql:"ts"`
}
