package binance

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/exchanges/binance/api"
)

type watcher struct {
	binance                      *binance
	logger                       zerolog.Logger
	market                       common.Market
	onChange                     func(data interface{})
	query                        *types.DepthQuery
	atomicOrderbook              atomic.Value //common.BidsAsks
	chDepth                      chan struct{}
	chDiff                       chan struct{}
	chDepthStop, chPartDepthStop chan struct{}
	tickers                      string
}

func (b *binance) startWatcher(market common.Market, onChange func(data interface{})) {
	watcher := watcher{
		binance:  b,
		logger:   log.With().Str("module", "watcher").Str("exchange", b.GetName()).Str("market", market.String()).Logger(),
		market:   market,
		onChange: onChange,
		query:    types.NewDepthQuery(market.BaseAsset.Symbol.String(), market.QuoteAsset.Symbol.String()),
		chDepth:  make(chan struct{}),
		chDiff:   make(chan struct{}),
		tickers:  fmt.Sprintf("%s%s", market.BaseAsset.Ticker, market.QuoteAsset.Ticker),
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
	w.subsPartialDepth()
	w.check()
	ticker := time.NewTicker(c.BINANCE_ORDERBOOK_CHECK_MS * time.Millisecond)
	for {
		select {
		case <-common.Stop:
			w.logger.Info().Msg("bdex exchange loop stopped on common.Stop signal")
			close(w.chDepthStop)
			close(w.chPartDepthStop)
			return
		case <-ticker.C:
			//w.logger.Info().Msg("refresh ticker")
			w.check()
		case <-w.chDepth:
			w.logger.Debug().Msg("resubscribe Depth on error")
			if !w.subsDepth() {
				time.AfterFunc(c.RESUBSCRIBE_DEPTH_SEC*time.Second, func() { w.chDepth <- struct{}{} })
			}
		case <-w.chDiff:
			w.logger.Debug().Msg("resubscribe Diff on error")
			if !w.subsPartialDepth() {
				time.AfterFunc(c.RESUBSCRIBE_DEPTH_SEC*time.Second, func() { w.chDiff <- struct{}{} })
			}
		}
	}
}
func (w *watcher) check() {
	ob, err := w.readOrderbook()
	if err != nil {
		w.logger.Error().Err(err).Msg("readOrderbook failed")
		return
	} // if error, no problem, we continue, maybe next time we will read the orderbook properly
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
	/*
			https://github.com/binance-exchange/binance-official-api-docs/blob/master/web-socket-streams.md#partial-book-depth-streams
			https://www.binance.com/en/support/articles/360032916632-Binance-WebSocket-Order-Book-Updates-Now-10x-Faster

			How to manage a local order book correctly
			Open a stream to wss://stream.binance.com:9443/ws/bnbbtc@depth.
			Buffer the events you receive from the stream.
			Get a depth snapshot from https://api.binance.com/api/v3/depth?symbol=BNBBTC&limit=1000 .
			Drop any event where u is <= lastUpdateId in the snapshot.
			The first processed event should have U <= lastUpdateId+1 AND u >= lastUpdateId+1.
			While listening to the stream, each new event's U should be equal to the previous event's u+1.
			The data in each event is the absolute quantity for a price level.
			If the quantity is 0, remove the price level.
			Receiving an event that removes a price level that is not in your local order book can happen and is normal.

		1) <symbol>@depth@100ms:

		Example: wss://stream.binance.com:9443/ws/bnbusdt@depth@100ms

		2) <symbol>@depth<levels>@100ms (<levels> = 5, 10, or 20):

	*/
	var i int
	var err error
	for i = 0; i < 10; i++ {
		_, w.chDepthStop, err = api.WsDepthServe(w.tickers, w.onDepthEvent,
			func(err error) {
				w.logger.Debug().Err(err).Msg("onError - DepthEvent")
				w.chDepth <- struct{}{}
			},
		)
		if err == nil {
			break
		} else {
			w.logger.Info().Err(err).Msg("WsDepthServe retry after 1000ms")
		}
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		w.logger.Error().Err(err).Msg("WsDepthServe failed")
		return false
	}
	if i > 0 {
		w.logger.Info().Int("try", i).Msg("WsDepthServe success")
	}
	return true
}
func (w *watcher) subsPartialDepth() bool {
	var err error
	for i := 0; i < 10; i++ {
		_, w.chPartDepthStop, err = api.WsPartialDepthServe(w.tickers, "50", w.onPartialDepthEvent,
			func(err error) {
				w.logger.Debug().Err(err).Msg("onError - DepthEvent")
				w.chDepth <- struct{}{}
			},
		)
		if err == nil {
			break
		} else {
			w.logger.Info().Err(err).Msg("WsPartialDepthServe retry after 1000ms")
		}
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		w.logger.Error().Err(err).Msg("WsPartialDepthServe failed")
		return false
	}
	return true
}
func (w *watcher) onPartialDepthEvent(event *api.WsPartialDepthEvent) {
	oldi := w.atomicOrderbook.Load()
	if oldi == nil {
		return
	}
	old := oldi.(common.BidsAsks)
	if old.IsEmpty() { // wait for full ordebook
		return
	}
	debug_check_orderbook(old, w.logger)
	new := common.BidsAsks{
		Bids: updateAndSortBidStrEntries(old.Bids, event.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: updateAndSortAskStrEntries(old.Asks, event.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	debug_check_orderbook(new, w.logger)
	w.atomicOrderbook.Store(new)
	w.fireNewOrderbook(new)
}
func (w *watcher) onDepthEvent(event *api.WsDepthEvent) {
	oldi := w.atomicOrderbook.Load()
	if oldi == nil {
		return
	}
	old := oldi.(common.BidsAsks)
	debug_check_orderbook(old, w.logger)
	if old.IsEmpty() { // wait for full ordebook
		w.logger.Info().Msg("Orderbook still empty")
		return
	}
	new := common.BidsAsks{
		Bids: updateAndSortBidStrEntries(old.Bids, event.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: updateAndSortAskStrEntries(old.Asks, event.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	debug_check_orderbook(new, w.logger)
	w.atomicOrderbook.Store(new)
	w.fireNewOrderbook(new)
}
func (w *watcher) readOrderbook() (common.BidsAsks, error) {
	return readOrderbook(w.binance, w.market, w.logger)
}
func readOrderbook(binance *binance, market common.Market, logger zerolog.Logger) (common.BidsAsks, error) {
	tickers := fmt.Sprintf("%s%s", market.BaseAsset.Ticker, market.QuoteAsset.Ticker)
	depth, err := binance.api.NewDepthService().Symbol(tickers).Limit(20).Do(context.Background())
	if err != nil {
		return common.EmptyBidsAsks, errors.Wrap(err, "failed to call dex.GetDepth")
	}
	ob := common.BidsAsks{
		Bids: convertAndSortBidStrEntries(depth.Bids, func(a, b common.Uint) bool { return a.GT(b) }),
		Asks: convertAndSortAskStrEntries(depth.Asks, func(a, b common.Uint) bool { return a.LT(b) }),
	}
	debug_check_orderbook(ob, logger)
	return ob, nil
}
func convertAndSortBidStrEntries(entries []api.Bid, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	oe := make(common.OrderbookEntries, len(entries))
	for i, bid := range entries {
		oe[i] = common.PA{}
		oe[i].Price = common.NewUintFromString(bid.Price)
		oe[i].Amount = common.NewUintFromString(bid.Quantity)
	}
	sort.SliceStable(oe, func(i, j int) bool {
		return compare(oe[i].Price, oe[j].Price)
	})
	return oe
}
func convertAndSortAskStrEntries(entries []api.Ask, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	oe := make(common.OrderbookEntries, len(entries))
	for i, ask := range entries {
		oe[i] = common.PA{}
		oe[i].Price = common.NewUintFromString(ask.Price)
		oe[i].Amount = common.NewUintFromString(ask.Quantity)
	}
	sort.SliceStable(oe, func(i, j int) bool {
		return compare(oe[i].Price, oe[j].Price)
	})
	return oe
}
func updateAndSortBidStrEntries(old common.OrderbookEntries, new []api.Bid, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	converted := make([]api.Ask, len(new))
	for i, entry := range new {
		converted[i].Price = entry.Price
		converted[i].Quantity = entry.Quantity
	}
	return updateAndSortStrEntries(old, converted, compare)
}
func updateAndSortAskStrEntries(old common.OrderbookEntries, new []api.Ask, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	return updateAndSortStrEntries(old, new, compare)
}
func updateAndSortStrEntries(old common.OrderbookEntries, new []api.Ask, compare func(common.Uint, common.Uint) bool) common.OrderbookEntries {
	add := make(common.OrderbookEntries, 0, 20)
	for _, ne := range new {
		np := common.NewUintFromString(ne.Price)
		na := common.NewUintFromString(ne.Quantity)
		i := sort.Search(len(old), func(i int) bool { return old[i].Price.GTE(np) })
		if i < len(old) && old[i].Price == np {
			old[i].Amount = na
		} else {
			ob := common.PA{}
			ob.Price = np
			ob.Amount = na
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
