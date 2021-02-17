package store

//go:generate mockgen -destination mock_store.go -package store . Store
import (
	//	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/models"
)

// Store represents methods required by Bot to store and load data from internal data store.
type Store interface {
	Ping() error

	InsertTrade(record models.Trade) error

	GetStats() (models.Stats, error)
	GetTrades() (models.Trades, error)
}
