package model

import "time"

type Kline struct {
	tableName struct{}  `sql:"coinbene_kline"`
	CreatedAt time.Time `sql:"created_at"`
	UpdatedAt time.Time `sql:"updated_at"`
	High      float64   `sql:"high"`
	Low       float64   `sql:"low"`
	Open      float64   `sql:"open"`
	Close     float64   `sql:"close"`
	Volume    float64   `sql:"volume"`
	Gap       string    `sql:"gap"`
	Ts        time.Time `sql:"ts"`
}
