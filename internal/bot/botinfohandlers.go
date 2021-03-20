package bot

import (
	"time"
	"gitlab.com/zlyzol/skadi/internal/models"
	"gitlab.com/zlyzol/skadi/internal/common"
)

// GetStartTime returns bot start time
func (bot *Bot) GetStartTime() time.Time {
	return bot.startTime
}

// GetHealth returns health status of Bot's crucial units.
func (bot *Bot) GetHealth() *models.HealthStatus {
	ok := func (err error) string {if err == nil { return "Running" } else { return "Error: " + err.Error() }}
	return &models.HealthStatus{
		Database: ok(bot.store.Ping()),
		Bot: ok(bot.Ping()), 
	}
}

// GetStart starts bot's hunters.
func (bot *Bot) GetStart() *models.StringResult {
	bot.StartHunters()
	return &models.StringResult{
		Result: bot.GetStatus(),
	}
}

// GetStop sttops bot's hunters.
func (bot *Bot) GetStop() *models.StringResult {
	bot.StopHunters()
	return &models.StringResult{
		Result: bot.GetStatus(),
	}
}

// GetStats returns some statistic data of network
func (bot *Bot) GetStats() (*models.Stats, error) {
	stats, err := bot.store.GetStats()
	stats.TimeRunning = time.Since(bot.GetStartTime()).String()
	stats.IpAddress = common.GetPublicIP()
	stats.Status = bot.GetStatus()
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

// GetWallet returns trade list
func (bot *Bot) GetWallet() (*models.Wallet, error) {
	result := bot.WalletChecker.GetWallet()
	return &result, nil
}

