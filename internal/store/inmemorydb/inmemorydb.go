package inmemorydb

import (
	"sync"
//	"sync/mutex"

	"gitlab.com/zlyzol/skadi/internal/models"
)

type InMemoryDb struct {
	mux			sync.Mutex
	totalYield	int64
	trades 		models.Trades
}

func NewClient() (*InMemoryDb, error) {
	return &InMemoryDb{
		trades: make(models.Trades, 0),
	}, nil
}

func (m *InMemoryDb) Ping() error {
	return nil
}
func (m *InMemoryDb) GetStats() (models.Stats, error) {
	result := models.Stats{}
	result.TradeCount = len(m.trades)
	if result.TradeCount > 0 {
		for _, trade := range m.trades {
			result.AvgPNL += trade.PNL
			result.TotalVolume += trade.AmountIn
			result.TotalYield += trade.AmountOut - trade.AmountIn
		}
		result.AvgPNL = result.AvgPNL / float64(result.TradeCount)
		result.AvgTrade = result.TotalVolume / float64(result.TradeCount)
	}
	return result, nil
}

func (m *InMemoryDb) GetTrades() (models.Trades, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	limit := 100
	result := make(models.Trades, 0, limit)
	cnt := 0
	for i := len(m.trades) - 1; i >= 0; i-- {
		result = append(result, m.trades[i])
		cnt++
		if cnt >= 100 {
			break
		}
	}
	return result, nil
}

func (m *InMemoryDb) InsertTrade(trade models.Trade) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.trades = append(m.trades, trade)
	return nil
}
