package models

import (
	"time"
)

type Trade struct {
	Time      time.Time `json:"time"`
	Asset     string    `json:"asset"`
	AmountIn  float64   `json:"amountIn"`
	AmountOut float64   `json:"amountOut"`
	PNL       float64   `json:"pnl"`
	Debug     string	`json:"debug"`
}

type Trades []Trade