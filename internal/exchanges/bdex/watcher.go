package bdex

import (
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/binance-chain/go-sdk/client/websocket"
	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
)

type watcher struct {
	bdex            *bdex
	logger          zerolog.Logger
	market          common.Market
	onChange        func(data interface{})
	atomicOrderbook atomic.Value //common.BidsAsks
	chDepth         chan struct{}
	chDiff          chan struct{}
}

func (b *bdex) startWatcher(market common.Market, onChange func(data interface{})) {
	watcher := watcher{
		bdex:     b,
		logger:   log.With().Str("module", "watcher").Str("exchange", b.GetName()).Str("market", market.String()).Logger(),
		market:   market,
		onChange: onChange,
		chDepth:  make(chan struct{}),
		chDiff:   make(chan struct{}),
	}
	ob, err := watcher.readOrderbook()
	if err == nil {
		watcher.atomicOrderbook.Store(ob)
	} else {
		watcher.logger.Error().Err(err).Msg("watcher first orderbook read failed")
		panic(0)
	}
	go watcher.watch()
}
func (w *watcher) watch() {
	w.logger.Info().Msg("watcher loop started")
	w.subsDepth()
	w.subsDiff()
	w.check()
	ticker := time.NewTicker(c.BDEX_ORDERBOOK_CHECK_MS * time.Millisecond)
	for {
		select {
		case <-common.Stop:
			w.logger.Info().Msg("bdex exchange loop stopped on common.Stop signal")
			return
		case <-ticker.C:
			w.check()
		case <-w.chDepth:
			w.logger.Debug().Msg("resubscribe Depth on error")
			if !w.subsDepth() {
				time.AfterFunc(c.RESUBSCRIBE_DEPTH_SEC*time.Second, func() { w.chDepth <- struct{}{} })
			}
		case <-w.chDiff:
			w.logger.Debug().Msg("resubscribe Diff on error")
			if !w.subsDiff() {
				time.AfterFunc(c.RESUBSCRIBE_DEPTH_SEC*time.Second, func() { w.chDiff <- struct{}{} })
			}
		}
	}
}
func (w *watcher) check() {
	ob, err := w.readOrderbook()
	if err != nil {
		if strings.Contains(err.Error(), "API rate limit exceeded") {
			time.Sleep(c.DEX_API_RATE_SLEEP_MS * time.Millisecond)
			ob, err = w.readOrderbook()
		}
		if err != nil { // if error, no problem, we continue, maybe next time we will read the orderbook properly
			w.logger.Error().Err(err).Msg("readOrderbook failed")
			return
		}
	} else {
		w.logger.Debug().Msg("readOrderbook succeeded")
	}
	debug_check_orderbook(ob, w.logger)
	oldi := w.atomicOrderbook.Load()
	if oldi == nil {
		debug_check_orderbook(ob, w.logger)
		w.atomicOrderbook.Store(ob)
		w.fireNewOrderbook(ob)
	} else {
		old := oldi.(common.BidsAsks)
		if !old.IsEqualTrunc(ob) {
			debug_check_orderbook(ob, w.logger)
			w.atomicOrderbook.Store(ob)
			w.fireNewOrderbook(ob)
		}
	}
}
func debug_check_orderbook(ob common.BidsAsks, logger zerolog.Logger) {
	ob.CheckOK()
}
func (w *watcher) fireNewOrderbook(ob common.BidsAsks) {
	debug_check_orderbook(ob, w.logger)
	w.onChange(ob)
}
func (w *watcher) subsDepth() bool {
	var i int
	var err error
	for i = 0; i < 10; i++ {
		err = w.bdex.dex.SubscribeMarketDepthEvent(w.market.BaseAsset.Symbol.String(), w.market.QuoteAsset.Symbol.String(), common.Stop, w.onMarketDepthEvent,
			func(err error) {
				w.logger.Debug().Err(err).Msg("onError - MarketDepthEvent")
				w.chDepth <- struct{}{}
			},
			func() {
				w.logger.Info().Err(err).Msg("MarketDepthEvent closed")
			})
		if err == nil {
			break
		} else {
			w.logger.Debug().Err(err).Msg("SubscribeMarketDepthEvent retry after 1000ms")
		}
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		w.logger.Error().Err(err).Msg("10x SubscribeMarketDepthEvent failed - we will try after 10s")
		return false
	}
	if i > 0 {
		w.logger.Debug().Int("try", i).Msg("SubscribeMarketDiffEvent success")
	}
	return true
}
func (w *watcher) subsDiff() bool {
	var i int
	var err error
	for i = 0; i < 10; i++ {
		err = w.bdex.dex.SubscribeMarketDiffEvent(w.market.BaseAsset.Symbol.String(), w.market.QuoteAsset.Symbol.String(), common.Stop, w.onMarketDiffEvent,
			func(err error) {
				w.logger.Debug().Err(err).Msg("onError - MarketDiffEvent")
				w.chDepth <- struct{}{}
			},
			func() {
				w.logger.Info().Err(err).Msg("MarketDiffEvent closed")
			})
		if err == nil {
			break
		} else {
			w.logger.Debug().Err(err).Msg("SubscribeMarketDiffEvent retry after 1000ms")
		}
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		w.logger.Error().Err(err).Msg("10x SubscribeMarketDiffEvent failed")
		return false
	}
	if i > 0 {
		w.logger.Debug().Int("try", i).Msg("SubscribeMarketDiffEvent success")
	}
	return true
}
func (w *watcher) onMarketDiffEvent(event *websocket.MarketDeltaEvent) {
	oldi := w.atomicOrderbook.Load()
	if oldi == nil {
		return
	}
	old := oldi.(common.BidsAsks)
	if old.IsEmpty() { // wait for full ordebook
		return
	}
	new := common.BidsAsks{
		Bids: updateAndSortFx8Entries(old.Bids, event.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: updateAndSortFx8Entries(old.Asks, event.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	w.atomicOrderbook.Store(new)
	w.fireNewOrderbook(new)
}
func (w *watcher) onMarketDepthEvent(event *websocket.MarketDepthEvent) {
	ob := common.BidsAsks{
		Bids: convertAndSortFx8Entries(event.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: convertAndSortFx8Entries(event.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	w.atomicOrderbook.Store(ob)
	w.fireNewOrderbook(ob)
}
func (w *watcher) readOrderbook() (common.BidsAsks, error) {
	return readOrderbook(w.bdex, w.market)
}
func readOrderbook(bdex *bdex, market common.Market) (common.BidsAsks, error) {
	query := types.NewDepthQuery(market.BaseAsset.Symbol.String(), market.QuoteAsset.Symbol.String())
	depth, err := bdex.dex.GetDepth(query)
	if err != nil {
		return common.EmptyBidsAsks, errors.Wrap(err, "failed to call dex.GetDepth")
	}
	ob := common.BidsAsks{
		Bids: convertAndSortStrEntries(depth.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: convertAndSortStrEntries(depth.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	return ob, nil
}
func convertAndSortStrEntries(entries [][]string, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	oe := make(common.OrderbookEntries, len(entries))
	for i, pv := range entries {
		oe[i] = common.PA{}
		oe[i].Price = common.NewUintFromString(pv[0])
		oe[i].Amount = common.NewUintFromString(pv[1])
	}
	sort.SliceStable(oe, func(i, j int) bool {
		return compare(oe[i].Price, oe[j].Price)
	})
	return oe
}
func convertAndSortFx8Entries(entries [][]types.Fixed8, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	oe := make(common.OrderbookEntries, len(entries))
	for i, pv := range entries {
		oe[i] = common.PA{}
		oe[i].Price = common.NewUintFromFx8(pv[0])
		oe[i].Amount = common.NewUintFromFx8(pv[1])
	}
	sort.SliceStable(oe, func(i, j int) bool {
		return compare(oe[i].Price, oe[j].Price)
	})
	return oe
}
func updateAndSortFx8Entries(old common.OrderbookEntries, new [][]types.Fixed8, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	add := make(common.OrderbookEntries, 0, 20)
	for _, ne := range new {
		np := common.NewUintFromFx8(ne[0])
		na := common.NewUintFromFx8(ne[1])
		i := sort.Search(len(old), func(i int) bool { return old[i].Price.GTE(np) })
		if i < len(old) && old[i].Price == np {
			old[i].Amount = na
		} else {
			ob := common.PA{ Price: np, Amount: na }
			add = append(add, ob)
		}
	}
	mix := make(common.OrderbookEntries, 0, len(old)+len(add))
	for _, e := range old {
		if e.Amount != common.ZeroUint() {
			mix = append(mix, e)
		}
	}
	for _, e := range add {
		if e.Amount != common.ZeroUint() {
			mix = append(mix, e)
		}
	}
	sort.SliceStable(mix, func(i, j int) bool {
		return compare(mix[i].Price, mix[j].Price)
	})
	return mix
}
