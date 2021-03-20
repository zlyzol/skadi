package tcpool

import (
	"time"
	"sync"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/c"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var BASECHAIN = common.BNBChain

//var TCPools *tcpools

type tcpools struct {
	logger		zerolog.Logger
	acc			common.Account	// Account interface
	thor		*ThorAddr
	//pools		map[common.Ticker][]common.Pool
	markets		[]common.Market
	muxWatchers	sync.Mutex
	watchers	map[string]*watcher
	tickerWatchers	map[string][]*watcher
	tickerAssets	map[common.Ticker]common.Asset
}
func NewExchange(excf *config.ExchangeConfiguration, accf *config.AccountConfiguration, accs *common.Accounts) (common.Exchange, error) {
	//TCPools = nil
	logger := log.With().Str("module", "tcpool").Logger()
	seed, ok := excf.Parameters["seed_url"]
	if !ok {
		logger.Panic().Msg("seed_url parameter not found in exchange configuration")
	}
	tc := tcpools {
		logger:			logger,
		thor:			NewThorAddr(seed),
		watchers:		make(map[string]*watcher),
		tickerWatchers:	make(map[string][]*watcher),
	}
	tc.acc, ok = accs.Get(accf.Name)
	if !ok {
		tc.logger.Panic().Msg("bdex exchange & account must be configured before tcpool exchange")
	}
	err := tc.readMarketInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read readMarketInfo / pools")
	}
	//TCPools = &tc
	go tc.loop()
	return &tc, nil
}
func (tc *tcpools) loop() {
	ticker := time.NewTicker(c.ALLPOOL_CHECK_MS * time.Millisecond)
	for {
		select {
		case <-common.Stop:
			tc.logger.Info().Msg("bdex exchange loop stopped on common.Stop signal")
			return
		case <-ticker.C:
			tc.readAndCheckAllPool()
		}
	}
}
func (tc *tcpools) readAndCheckAllPool() {
	raws, err := tc.thor.getPools()
	if err != nil {
		tc.logger.Error().Err(err).Msg("getPools failed")
		return
	}
	for _, raw := range raws {
		asset, err := common.NewAsset(raw.Asset)
		if err != nil { 
			tc.logger.Error().Err(err).Str("asset", raw.Asset).Msg("NewAsset failed")
			continue
		}
		watcher, active := tc.watchers[asset.ChainTickerString()];
		if !active {
			continue
		}
		depths := common.Amounts{ BaseAmount: common.NewUintFromFx8String(raw.BalanceAsset), QuoteAmount: common.NewUintFromFx8String(raw.BalanceRune) }
		watcher.checkPool(depths)
	}
}
func (tc *tcpools) GetName() string {
	return "THORChain"
}
func (tc *tcpools) GetAccount() common.Account {
	return tc.acc
}
func (tc *tcpools) GetTrader(market common.Market) common.Trader {
	tc.logger.Panic().Msg("tcpool exchange doesn't have Trader")
	return nil
}
func (tc *tcpools) GetSwapper(market common.Market) common.Swapper {
	return NewSwapper(tc, market)
}
func (tc *tcpools) GetMarkets() []common.Market {
	return tc.markets
}
func (tc *tcpools) UpdateLimits(limits common.Amounts, market common.Market, oppositeExchange common.Exchange, side common.OrderSide) common.Amounts {
	perc99 := common.OneUint().MulUint64(99).QuoUint64(100)
	return common.Amounts{BaseAmount: perc99.Mul(limits.BaseAmount), QuoteAmount: perc99.Mul(limits.QuoteAmount)}
}
func (tc *tcpools) Subscribe(market common.Market, onChange func(interface{})) {
	w, new := tc.subscribeToWatcher(market.BaseAsset, func(depths common.Amounts) {
		onChange(depths)
	})
	if new {
		ticker := market.BaseAsset.Ticker.String()
		tc.muxWatchers.Lock()
		defer tc.muxWatchers.Unlock()
		tw, found := tc.tickerWatchers[ticker]
		if found {
			tc.tickerWatchers[ticker] = append(tw, w)
		} else {
			tc.tickerWatchers[ticker] = []*watcher{w}
		}
	}
}
func (tc *tcpools) GetCurrentOfferData(market common.Market) interface{} {
	tc.muxWatchers.Lock()
	watcher, active := tc.watchers[market.BaseAsset.ChainTickerString()];
	tc.muxWatchers.Unlock()
	if active {
		return watcher.refresh()
	}
	return common.ZeroAmounts()
}
// Pricer interface functions
func (tc *tcpools) GetRuneValueOf(amount common.Uint, asset common.Asset) common.Uint {
	return amount.Mul(tc.GetRunePriceOf(asset))
}
func (tc *tcpools) GetRunePriceOf(asset common.Asset) common.Uint {
	if asset.Chain.IsEmpty() || asset.Chain.IsUnknown() {
		asset, _ = common.NewChainAsset(common.BNBChainStr, asset.Symbol.String())
	}
	if common.IsRuneAsset(asset) { return common.OneUint() }
	tc.muxWatchers.Lock()
	defer tc.muxWatchers.Unlock()
	var depths common.Amounts
	watcher, found := tc.watchers[asset.ChainTickerString()]
	if found {
		depths = watcher.getPoolData()
		if depths.Equal(common.Amounts{}) {
			watcher.refresh()
			depths = watcher.getPoolData()
		}
	} else {
		depths = common.Amounts{}
		s := asset.Ticker.String()
		watchers, found := tc.tickerWatchers[s]
		if found {
			for _, watcher = range watchers {
				p := watcher.getPoolData()
				if p.QuoteAmount > depths.QuoteAmount {
					if p.Equal(common.Amounts{}) {
						watcher.refresh()
						p = watcher.getPoolData()
					}
					depths = p
				}
			}
		} else {
			tc.logger.Error().Msgf("cannot get rune price for not watched pool %s", asset)
			return common.ZeroUint()
		}
	}
	return depths.QuoteAmount.Quo(depths.BaseAmount)
}
func (tc *tcpools) GetPricer() common.Pricer {
	return tc
}
func (tc *tcpools) GetSymbolSolver() common.SymbolSolver {
	return tc
}
func (tc *tcpools) GetSymbolAsset(asset common.Asset) (common.Asset, error) {
	if a, found := tc.tickerAssets[asset.Ticker]; found {
		return a, nil
	}
	return common.EmptyAsset, errors.Errorf("cannot find full symbol for asset %s. That means there isn't active pool for this asset", asset)
}
