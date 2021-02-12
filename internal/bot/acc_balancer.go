package bot

import (

	//	"github.com/ethereum/go-ethereum/p2p"
	"github.com/pkg/errors"
	"sync/atomic"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	//	"gitlab.com/zlyzol/skadi/internal/exchange"
)

// AccBalancer structure
type AccBalancer struct {
	logger		zerolog.Logger
	balancing	map[common.Ticker]*int32 // 0 - not balancing, 1 - balancing
}
func NewAccBalancer() *AccBalancer {
	ab := AccBalancer{
		logger:		log.With().Str("module", "acc_balancer").Logger(),
		balancing:	make(map[common.Ticker]*int32, 100),
	}
	return &ab
}
type amounts struct { am1, am2 common.Uint }
func (a amounts) minmax() ( common.Uint, common.Uint) { if a.am1.GT(a.am2) { return a.am2, a.am1 } else { return a.am1, a.am2 }}
func shouldBalance(ticker common.Ticker, ams amounts) bool {
	min, max := ams.minmax()
	if max.IsZero() { return false }
	quo := min.Quo(max)
	lim := common.NewUintFromFloat((100.0 - c.DO_BALANCE_MIN_DIFF_PERC) / 100.0)
	if quo.GT(lim) { return false }
	asset, _ := common.NewAsset(ticker.String())
	if common.Oracle.GetRuneValueOf(max, asset).LT(common.NewUint(c.DO_BALANCE_MIN_AMOUNT_RUNE)) { return false }
	return true
}
func (ab *AccBalancer) addMarket(market common.Market) {
	ticker := market.BaseAsset.Ticker
	if _, found := ab.balancing[ticker]; !found {
		ab.balancing[ticker] = new(int32)
	}
	ticker = market.QuoteAsset.Ticker
	if _, found := ab.balancing[ticker]; !found {
		ab.balancing[ticker] = new(int32)
	}
}
func (ab *AccBalancer) balanceAll(acc1, acc2 common.Account) error {
	if acc1 == acc2 { return nil }
	bal1 := acc1.GetBalances()
	bal2 := acc2.GetBalances()
	balCount := len(bal1)
	if balCount < len(bal2) {
		balCount = len(bal2)
	}
	assets := make(map[common.Ticker]amounts, balCount)
	for ts, am := range bal1 {
		if am.IsZero() { continue }
		ticker, _ := common.NewTicker(ts)
		assets[ticker] = amounts{ am1: am, am2: 0 }
	}
	for ts, am := range bal2 {
		if am.IsZero() { continue }
		ticker, _ := common.NewTicker(ts)
		lr, found := assets[ticker]
		if found {
			assets[ticker] = amounts{ am1: lr.am1, am2: am }
		} else {
			assets[ticker] = amounts{ am1: 0, am2: am }
		}
	}
	for ticker, ams := range assets {
		if !shouldBalance(ticker, ams) { continue }
		err := ab.balance(acc1, acc2, ticker, ams)
		if err != nil {
			ab.logger.Error().Err(err).Msgf("cannot balance %s", ticker)
		}
	}
	return nil
}
func (ab *AccBalancer) balanceMarket(acc1, acc2 common.Account, market common.Market) error {
	err := ab.balanceTicker(acc1, acc2, market.BaseAsset.Ticker)
	if err != nil {
		return err
	}
	return ab.balanceTicker(acc1, acc2, market.QuoteAsset.Ticker)
}
func (ab *AccBalancer) balanceTicker(acc1, acc2 common.Account, ticker common.Ticker) error {
	if acc1 == acc2 { return nil }
	asset, _ := common.NewAsset(ticker.String())
	ams := amounts{
		am1: acc1.GetBalance(asset),
		am2: acc2.GetBalance(asset),
	}
	if !shouldBalance(ticker, ams) {
		return nil
	}
	return ab.balance(acc1, acc2, ticker, ams)
}
func (ab *AccBalancer) balance(acc1, acc2 common.Account, ticker common.Ticker, ams amounts) error {
	if acc1 == acc2 { return nil }
	addr, found := ab.balancing[ticker]
	if !found {
		return errors.Errorf("cannot find %s in account balancer", ticker)
	}
	if !atomic.CompareAndSwapInt32(addr, 0, 1) {
		return nil // already balancing
	}
	defer atomic.StoreInt32(addr, 0)
	var from, to common.Account
	if ams.am1 > ams.am2 { 
		from, to = acc1, acc2
	} else {
		from, to = acc2, acc1
	}
	min, max := ams.minmax()
	am := (max.Sub(min)).Quo(2)
	asset, _ := common.NewAsset(ticker.String())
	ab.logger.Info().Msgf("balancig %s %s from %s to %s", am, ticker, from.GetName(), to.GetName())
	_, _, err := from.Send(am, asset, to, true) 
	return errors.Wrapf(err, "failed balancig %s %s from %s to %s", am, ticker, from.GetName(), to.GetName())
}
