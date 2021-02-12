package bot

import (
//	"hash/adler32"
	"strings"
	"sort"

	"github.com/pkg/errors"
	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/exchanges/bdex"
	"gitlab.com/zlyzol/skadi/internal/exchanges/tcpool"
	"gitlab.com/zlyzol/skadi/internal/exchanges/binance"
)

// connectExchanges - connects/opens/configures exchange
func (bot *Bot) connectExchanges() error {
	bot.accounts = common.NewAccounts()
	bot.exchanges = make(map[string]common.Exchange, len(bot.cfg.Exchanges))
	keys := make([]string, 0, len(bot.cfg.Exchanges))
	for k := range bot.cfg.Exchanges {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		excf := bot.cfg.Exchanges[k]
		accf, ok := bot.cfg.Accounts[excf.AccountName]
		if !ok { 
			bot.logger.Panic().Msg("config error: account not found " +  excf.AccountName) 
		}
		if err := bot.connectExchange(&excf, &accf); err != nil { return err }
	}
	return nil
}

// connectExchange - connects/opens/configures exchange and opens wallets
func (bot *Bot) connectExchange(excf *config.ExchangeConfiguration, accf *config.AccountConfiguration) error {
	var ex common.Exchange
	var err error
	extype := strings.ToLower(excf.Type)
	switch extype {
	case "binance-dex":
		ex, err = bdex.NewExchange(excf, accf, bot.accounts)
	case "thorchain":
		ex, err = tcpool.NewExchange(excf, accf, bot.accounts)
		if err == nil { 
			common.Oracle = ex.(common.Pricer) 
			common.Symbols = ex.(common.SymbolSolver) 
		}
	case "binance":
		ex, err = binance.NewExchange(excf, accf, bot.accounts)
	case "bitmax":
		err = errors.New("exchange type " + extype + " not implemented yet")
		//ex, err = bitmax.NewExhange(excf, accf, bot.Accounts)
	default:
		bot.logger.Panic().Msg("unknown exchange type " + extype)
	}
	if err != nil {
		return err
	}
	bot.exchanges[extype] = ex
	return nil
}
