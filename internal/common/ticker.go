package common

import (
	"fmt"
	"strings"
)

const (
	BNBTicker     = Ticker("BNB")
	RuneTicker    = Ticker("RUNE")
	RuneA1FTicker = Ticker("RUNE-A1F")
	RuneB1ATicker = Ticker("RUNE-B1A")
)

type (
	Ticker  string
	Tickers []Ticker
)
var BTICKERS = [...]string{"BTTB", "BTCB", "MDAB", "NOIZB", "NPXB", "SPNDB", "TOMOB", "TRXB", "WINB"}

func NewTicker(ticker string) (Ticker, error) {
	noTicker := Ticker("")
	ticker = b(strings.ToUpper(ticker))
	if len(ticker) < 2 {
		return noTicker, fmt.Errorf("Ticker Error: Not enough characters")
	}

	if len(ticker) > 13 {
		return noTicker, fmt.Errorf("Ticker Error: Too many characters")
	}
	return Ticker(ticker), nil
}

func b(ticker string) string {
	for _, tb := range BTICKERS {
		if tb == ticker {
			return ticker[:len(ticker) - 1]
		}
	}
	return ticker
}

func (t Ticker) Equal(t2 Ticker) bool {
	
	s1 := t.String()
	s2 := t2.String()
	return strings.EqualFold(s1, s2)
}

func (t Ticker) IsEmpty() bool {
	return strings.TrimSpace(t.String()) == ""
}

func (t Ticker) String() string {
	// uppercasing again just incase someon created a ticker via Ticker("rune")
	return strings.ToUpper(string(t))
}

func IsBNB(ticker Ticker) bool {
	return ticker.Equal(BNBTicker)
}

func IsRune(ticker Ticker) bool {
	return ticker.Equal(RuneTicker) || ticker.Equal(RuneA1FTicker) || ticker.Equal(RuneB1ATicker)
}
