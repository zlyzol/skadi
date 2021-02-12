package models

type Stats struct {
	TradeCount  uint    `json:"tradeCount"`
	AvgPNL      float64 `json:"avgPNL"`
	TotalVolume float64 `json:"totalVolume"`
	AvgTrade    float64 `json:"avgTrade"`
	TimeRunning string  `json:"timeRunning"`
}