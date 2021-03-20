package models

import (
	"time"
)

type WalletAssetData struct {
	Asset	string	`json:"asset"`
	Start	float64	`json:"start"`
	Cur		float64	`json:"cur"`
	Plus	float64	`json:"plus"`
	PlusRune	float64	`json:"plusrune"`
 }

type Wallet struct {
	Time		time.Time	`json:"time"`
	PlusRune	float64		`json:"plusrune"`
	Assets		[]WalletAssetData   `json:"assets"`
}
