package models

type Stats struct {
	TradeCount  int     `json:"tradeCount"`
	AvgPNL      float64 `json:"avgPNL"`
	TotalVolume float64 `json:"totalVolume"`
	TotalYield  float64 `json:"totalYield"`
	AvgTrade    float64 `json:"avgTrade"`
	TimeRunning string  `json:"timeRunning"`
}