package tcpool

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/c"
)
type watcher struct {
	exchange	*tcpools
	logger		zerolog.Logger
	asset		common.Asset
	atomicPool	atomic.Value //common.Amounts
	refreshC	chan common.Amounts
	subscribers	common.ThreadSafeSlice
}
func (tc *tcpools) subscribeToWatcher(asset common.Asset, onChange func(depths common.Amounts)) (w *watcher, new bool) {
	a := asset.ChainTickerString()
	var found bool
	tc.muxWatchers.Lock()
	w, found = tc.watchers[a]
	tc.muxWatchers.Unlock()
	new = !found
	if new {
		w = &watcher{
			exchange:	tc,
			logger:		log.With().Str("module", "watcher").Str("exchange", tc.GetName()).Str("asset", asset.String()).Logger(),
			asset: 		asset,
			refreshC:	make(chan common.Amounts),
		}
		w.logger.Info().Msg("subscribe to new pool watcher")
		tc.muxWatchers.Lock()
		tc.watchers[a] = w
		tc.muxWatchers.Unlock()
	} else {
		w.logger.Info().Msg("subscribe to already running pool watcher")
	}
	go w.subcribe(onChange)
	if new {
		go w.watch()
	}
	return w, new
}
func (w *watcher) watch() {
	w.logger.Info().Msg("watcher loop started")
	w.check()
	ticker := time.NewTicker(c.TCPOOL_ORDERBOOK_CHECK_MS * time.Millisecond)
	for {
		select {
		case <-common.Quit:
			w.logger.Info().Msg("tcpool exchange loop stopped on common.Quit signal")
			return
		case depths := <-w.refreshC:
			if depths.IsEmpty() {
				panic("bad")
			}
			w.logger.Info().Msg("refreshC (GetPool)")
			w.checkPool(depths)
		case <-ticker.C:
			w.logger.Debug().Msg("refresh ticker 100s (GetPool)")
			w.check()
		}
	}
}
func (w *watcher) check() {
	depths, err := w.readPool()
	if err != nil {
		if err != THOR_API_RATE_LIMIT_ERROR {
			w.logger.Error().Err(err).Msg("readPool failed")
		}
		return
	} // if error, no problem, we continue, maybe next time we will read the orderbook properly
	if depths.IsEmpty() {
		panic("bad")
	}
	w.checkPool(depths)
}
func (w *watcher) checkPool(depths common.Amounts) {
	if depths.IsEmpty() {
		panic("bad")
	}
	oldi := w.atomicPool.Load()
	if oldi == nil {
		w.atomicPool.Store(depths)
		w.fireNewPool(depths)
	} else {
		old := oldi.(common.Amounts)
		if !old.Equal(depths) {
			w.atomicPool.Store(depths)
			w.fireNewPool(depths)
		}
	}
}
func (w *watcher) fireNewPool(depths common.Amounts) {
	if depths.IsEmpty() {
		panic("bad")
	}
	if depths.IsEmpty() {
		return
	}
	w.subscribers.Iter(func(worker *common.ThreadSafeSliceWorker) {
		worker.ListenerC <- depths
	})
}
func (w *watcher) readPool() (common.Amounts, error) {
	raw, err := w.exchange.thor.getPool(w.asset.Chain.String() + "." + w.asset.Symbol.String())
	if err != nil {
		w.logger.Error().Err(err).Msg("readPool error - ignore")
		return common.ZeroAmounts(), err
	}
	depths := common.Amounts{ BaseAmount: common.NewUintFromFx8String(raw.BalanceAsset), QuoteAmount: common.NewUintFromFx8String(raw.BalanceRune) }
	/*
	w.fireNewPool(depths)
	*/
	return depths, nil
}
func (w *watcher) refresh() common.Amounts {
	depths, err := w.readPool()
	if err == nil {
		w.refreshC <-depths
	}
	return depths
}
func (w *watcher) getPoolData() common.Amounts {
	pooli := w.atomicPool.Load()
	if pooli == nil {
		return common.Amounts{}
	} else {
		return pooli.(common.Amounts)
	}
}
func (w *watcher) subcribe(onChange func(common.Amounts)) {
	worker := common.ThreadSafeSliceWorker{ ListenerC: make(chan interface{}, 10) }
	w.subscribers.Push(&worker)
	for {
		select {
		case <-common.Quit:
			return
		case depths := <-worker.ListenerC:
			onChange(depths.(common.Amounts))
		}
	}
}
