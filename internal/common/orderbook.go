package common

import (
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

type OrderbookEntries []PA
type BidsAsks struct {
	Bids OrderbookEntries
	Asks OrderbookEntries
}
var EmptyBidsAsks = BidsAsks{Bids: OrderbookEntries{}, Asks: OrderbookEntries{}}
type Orderbook struct { // implements Offer interface
	exchange   Exchange
	market     Market
	orderbooks atomic.Value // BidsAsks
	// cached:
	Bids OrderbookEntries
	Asks OrderbookEntries
}

func (oes OrderbookEntries) RepareConsistency() error {
	for i := len(oes) - 1; i >= 0; i-- {
		oe := oes[i]
		if oe.Price.IsZero() || oe.Amount.IsZero() {
			return fmt.Errorf("Price or amount in orderbook is zero")
		}
	}
	return nil
}
func NewOrderbook(exchange Exchange, market Market) *Orderbook {
	ob := &Orderbook{
		exchange: exchange,
		market:   market,
		Bids:     OrderbookEntries{},
		Asks:     OrderbookEntries{},
	}
	ob.orderbooks.Store(EmptyBidsAsks)
	return ob
}
func (ob *Orderbook) AtomicSetData(obs BidsAsks) {
	if obs.CheckOK() {
		ob.orderbooks.Store(obs)
	} else {
		log.Error().Str("ob market", ob.market.String()).Msg("OB SetData not OK")
		obs.CheckOK()
	}
}
func (ob *Orderbook) AtomicGetData() BidsAsks {
	obs := ob.orderbooks.Load().(BidsAsks)
	if !obs.CheckOK() {
		log.Error().Str("ob market", ob.market.String()).Msg("OB GetData not OK")
		return EmptyBidsAsks
	}
	return obs
}
func (ob *Orderbook) UpdateCache() {
	loaded := ob.AtomicGetData()
	if loaded.CheckOK() {
		ob.Bids = loaded.Bids
		ob.Asks = loaded.Asks
	} else {
		log.Error().Str("ob market", ob.market.String()).Msg("OB UpdateCache not OK")
	}
}
func (ob *Orderbook) IsEmpty() bool {
	is := ob.Bids == nil || ob.Asks == nil || len(ob.Bids) == 0 || len(ob.Asks) == 0
	return is
}
func (ob *Orderbook) GetEntry(tradeType int8, i int) PA {
	if ob.IsEmpty() {
		return PA0
	}
	var ba []PA
	if tradeType == OfferSide.BID {
		ba = ob.Bids
	} else {
		ba = ob.Asks
	}
	if len(ba) <= i {
		return PA0
	}
	return ba[i]
	//return fmt.Sprintf("[%f@%f]", ba[i].Amount, ba[i].Price)
}
func (ob *Orderbook) GetAvgFor(tt int8, pa PA) PA {
	if ob.IsEmpty() {
		return PA0
	}
	var comp func(a, b Uint) bool
	var ba []PA
	if pa.Price == ZeroUint() {
		comp = func(a, b Uint) bool {
			return true
		}
	} else if tt == OfferSide.BID {
		ba = ob.Bids
		comp = func(a, b Uint) bool {
			return a.GT(b)
		}
	} else {
		ba = ob.Asks
		comp = func(a, b Uint) bool {
			return a.LT(b)
		}
	}
	if len(ba) == 0 {
		return PA0
	}
	if pa == PA0 {
		return ba[0]
	}
	var res PA
	for i := 0; i < len(ba) && comp(ba[i].Price, pa.Price); i++ {
		if pa.Amount.GT(ZeroUint()) && ba[i].Amount.GT(pa.Amount) {
			plusAmount := pa.Amount.Sub(res.Amount)
			res = PA{Price: (res.Price.Mul(res.Amount).Add(ba[i].Price.Mul(plusAmount))).Quo((res.Amount.Add(plusAmount))),
				Amount: res.Amount.Add(plusAmount)}
			break
		}
		plusAmount := ba[i].Amount.Sub(res.Amount)
		res = PA{Price: (res.Price.Mul(res.Amount).Add(ba[i].Price.Mul(plusAmount))).Quo(ba[i].Amount),
			Amount: ba[i].Amount}
	}
	return res
}
func (ob *Orderbook) GetMarket() Market {
	return ob.market
}
func (ob *Orderbook) GetExchange() Exchange {
	return ob.exchange
}
func (ob *Orderbook) GetOrderbook() *Orderbook {
	return ob
}
func (ob *Orderbook) Merge(Offer) (Offer, error) {
	panic("Orderbook.Merge not implemented")
}
func (ob *Orderbook) GetPool() *Pool {
	return nil
}
func (o *Orderbook) GetEntries() (bids OrderbookEntries, asks OrderbookEntries) {
	return o.Bids, o.Asks
}
func (o *Orderbook) GetTrader() Trader {
	return o.exchange.GetTrader(o.market)
}
func (o BidsAsks) Equal(o2 BidsAsks) bool {
	if len(o.Bids) != len(o2.Bids) || len(o.Asks) != len(o2.Asks) {
		return false
	}
	for i, v := range o.Bids {
		if v != o2.Bids[i] {
			return false
		}
	}
	for i, v := range o.Asks {
		if v != o2.Asks[i] {
			return false
		}
	}
	return true
}
func (o BidsAsks) IsEqualTrunc(o2 BidsAsks) bool {
	Min := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}
	blen := Min(len(o.Bids), len(o2.Bids))
	alen := Min(len(o.Asks), len(o2.Asks))
	if blen < 10 || alen < 10 {
		return false
	}
	for i := 0; i < blen; i++ {
		if o.Bids[i] != o2.Bids[i] {
			return false
		}
	}
	for i := 0; i < alen; i++ {
		if o.Asks[i] != o2.Asks[i] {
			return false
		}
	}
	return true
}
func (o BidsAsks) IsEmpty() bool {
	is := o.Bids == nil || o.Asks == nil || len(o.Bids) == 0 || len(o.Asks) == 0
	return is
}
func (o BidsAsks) CheckOK() bool {
	if o.IsEmpty() { return true }
	if !o.Bids.CheckOK() { return false }
	if !o.Asks.CheckOK() { return false }
	return true
}
func (oes OrderbookEntries) CheckOK() bool {
	debug_print_orderbook := func(oes OrderbookEntries) {
		s := "Entries:"
		for i, oe := range oes {
			s = fmt.Sprintf("%s\n%v: %s", s, i, oe)
		}
	}
	for i, oe := range oes {
		if oe.Price.IsZero() || oe.Amount.IsZero() {
			log.Error().Int("i", i).Msg("Price or amount in orderbook is zero")
			debug_print_orderbook(oes)
			//panic("see log")
			return false
		}
	}
	return true
}

func orderbook_interface_test() {
	var _ Offer = &Orderbook{}
}
