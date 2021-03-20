package binance

import (
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/exchanges/binance/api"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type binance struct {
	logger  zerolog.Logger
	acc     common.Account // Account interface
	api     *api.Client
	markets marketInfoMap
	chains  chainInfoMap
}

func NewExchange(excf *config.ExchangeConfiguration, accf *config.AccountConfiguration, accs *common.Accounts) (common.Exchange, error) {
	b := binance{
		logger: log.With().Str("module", "binance").Logger(),
	}
	var err error
	// account, keys, addresses
	b.acc, b.api, err = b.NewAccount(accf, accs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exchange account")
	}
	err = b.readMarketInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read readMarketInfo")
	}
	return &b, nil
}
func (b *binance) GetName() string {
	return "Binance"
}
func (b *binance) GetAccount() common.Account {
	return b.acc
}
func (b *binance) GetTrader(market common.Market) common.Trader {
	return NewTrader(b, market)
}
func (b *binance) GetSwapper(market common.Market) common.Swapper {
	b.logger.Panic().Msg("binance exchange doesn't have swapper")
	return nil
}

func (b *binance) GetMarkets() []common.Market {
	res := make([]common.Market, 0, len(b.markets))
	for _, m := range b.markets {
		res = append(res, m.market)
	}
	return res
}
func (b *binance) UpdateLimits(limits common.Amounts, market common.Market, oppositeExchange common.Exchange, side common.OrderSide) common.Amounts {
	/*
		maxRune := common.NewUint(c.MAX_ARB_RUNE)
		price := common.Oracle.GetRunePriceOf(market.BaseAsset)
		if price.Mul(limits.BaseAmount).GT(maxRune) { //2 * 300 > 200 -> 600/200=3 ... 300/3 /// pr * am > max -> am*max / pr*am
			b.logger.Info().Str("amount", limits.BaseAmount.String()).Str("shrinked to", limits.BaseAmount.Mul(maxRune).Quo(price.Mul(limits.BaseAmount)).String()).Msg("asset limits shrinked due to maxRune constant")
			limits.BaseAmount = limits.BaseAmount.Mul(maxRune).Quo(price.Mul(limits.BaseAmount))
		}
		price = common.Oracle.GetRunePriceOf(market.QuoteAsset)
		if price.Mul(limits.QuoteAmount).GT(maxRune) { //2 * 300 > 200 -> 600/200=3 ... 300/3 /// pr * am > max -> am*max / pr*am
			b.logger.Info().Str("amount", limits.BaseAmount.String()).Str("shrinked to", limits.QuoteAmount.Mul(maxRune).Quo(price.Mul(limits.QuoteAmount)).String()).Msg("quoted limits shrinked due to maxRune constant")
			limits.QuoteAmount = limits.QuoteAmount.Mul(maxRune).Quo(price.Mul(limits.QuoteAmount))
		}
	*/
	perc99 := common.OneUint().MulUint64(99).QuoUint64(100)
	return common.Amounts{BaseAmount: perc99.Mul(limits.BaseAmount), QuoteAmount: perc99.Mul(limits.QuoteAmount)}
}
func (b *binance) Subscribe(market common.Market, onChange func(data interface{})) {
	b.startWatcher(market, onChange)
}
func (b *binance) GetCurrentOfferData(market common.Market) interface{} {
	ob, err := readOrderbook(b, market, log.With().Str("module", "binance").Logger())
	if err != nil {
		return common.EmptyBidsAsks
	}
	return ob
}
