package bot

import (
	"time"
	"gitlab.com/zlyzol/skadi/internal/models"
)

// GetStartTime returns bot start time
func (bot *Bot) GetStartTime() time.Time {
	return bot.startTime
}

// GetHealth returns health status of Bot's crucial units.
func (bot *Bot) GetHealth() *models.HealthStatus {
	return &models.HealthStatus{
		Database: bot.store.Ping() == nil,
	}
}

// GetStats returns some statistic data of network
func (bot *Bot) GetStats() (*models.Stats, error) {
	stats, err := bot.store.GetStats()
	stats.TimeRunning = time.Since(bot.GetStartTime()).String()
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetTrades returns trade list
func (bot *Bot) GetTrades() (*models.Trades, error) {
	trades, err := bot.store.GetTrades()
	if err != nil {
		return nil, err
	}
	return &trades, nil
}

