package bdex

import (
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/config"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/binance-chain/go-sdk/client"
	//	"github.com/binance-chain/go-sdk/types/msg"
	//	"github.com/binance-chain/go-sdk/types/tx"
)

var BASECHAIN = common.BNBChain

type bdex struct {
	logger  		zerolog.Logger
	acc     		common.Account   // Account interface support
	dex     		client.DexClient // Binance DEX client
	markets			marketInfoMap
}

func NewExchange(excf *config.ExchangeConfiguration, accf *config.AccountConfiguration, accs *common.Accounts) (common.Exchange, error) {
	b := bdex{
		logger: log.With().Str("module", "bdex").Logger(),
	}
	var err error
	// account, keys, addresses
	b.acc, err = b.NewAccount(accf, accs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exchange account")
	}
	err = b.readMarketInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read readMarketInfo")
	}
	/*
		cdc := tx.Cdc
		cdc.RegisterConcrete(CreateOrderMsg{}, "bdex/NewOrder", nil)
		cdc.RegisterConcrete(CreateMarketOrderMsg{}, "bdex/NewMarketOrder", nil)
		cdc = msg.MsgCdc
		cdc.RegisterConcrete(CreateOrderMsg{}, "bdex/NewOrder", nil)
		cdc.RegisterConcrete(CreateMarketOrderMsg{}, "bdex/NewMarketOrder", nil)
	*/
	return &b, nil
}
func (b *bdex) GetName() string {
	return "Binance DEX"
}
func (b *bdex) CanWait() bool { // true if it is not necessary to make the trade immediately
	return false
}
func (b *bdex) GetAccount() common.Account {
	var a common.Account
	a = b.acc
	return a
}
func (b *bdex) GetTrader(market common.Market) common.Trader {
	return NewTrader(b, market)
}
func (b *bdex) GetSwapper(market common.Market) common.Swapper {
	b.logger.Panic().Msg("bdex exchange doesn't have swapper")
	return nil
}
func (b *bdex) GetMarkets() []common.Market {
	res := make([]common.Market, 0, len(b.markets))
	for _, m := range b.markets {
		res = append(res, m.market)
	}
	return res
}
func (b *bdex) UpdateLimits(limits common.Amounts, market common.Market, oppositeExchange common.Exchange, side common.OrderSide) common.Amounts {
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
func (b *bdex) Subscribe(market common.Market, onChange func(data interface{})) {
	b.startWatcher(market, onChange)
}
func (b *bdex) GetCurrentOfferData(market common.Market) interface{} {
	ob, err := readOrderbook(b, market)
	if err != nil {
		return common.EmptyBidsAsks
	}
	return ob
}
