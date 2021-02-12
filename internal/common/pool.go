package common

import (
	"fmt"
	"sync/atomic"
)

// singlePool - single pool - supports Offer interface
type singlePool struct {
	market		Market
	cache		Amounts
	flipped		bool
	depths		atomic.Value // Amounts
}
// Pool - single pool or dual pool - supports Offer and Pool interface
type Pool struct {
	exchange	Exchange
	market		Market
	pools		[]singlePool
}
func NewSinglePool(exchange Exchange, market Market) *Pool {
	pool := &Pool{
		exchange:	exchange,
		market:		market,
		pools:		[]singlePool{{ market: market, cache: ZeroAmounts(), flipped: false }},
	}
	pool.pools[0].depths.Store(ZeroAmounts())
	return pool
}
func NewSinglePoolFlip(exchange Exchange, market Market, flipped bool) *Pool {
	var flipMarket Market
	if flipped {
		flipMarket = NewMarket(market.QuoteAsset, market.BaseAsset)
	} else {
		flipMarket = market
	}
	pool := &Pool{
		exchange:	exchange,
		market:		flipMarket,
		pools:		[]singlePool{{ market: flipMarket, cache: ZeroAmounts(), flipped: flipped }},
	}
	pool.pools[0].atomicSetData(ZeroAmounts())
	return pool
}
func (pool *singlePool) atomicSetData(depths Amounts) {
	if pool.flipped { depths.Flip() }
	pool.depths.Store(depths)
}
func (pool *singlePool) atomicGetData() Amounts {
	depths := pool.depths.Load().(Amounts)
	return depths
}
func (pool *singlePool) UpdateCache() {
	pool.cache = pool.atomicGetData()
}
func (pool *singlePool) base() Asset {
	return pool.market.BaseAsset
}
func (pool *singlePool) quote() Asset {
	return pool.market.QuoteAsset
}
func (pool *singlePool) baseDepth() Uint {
	return pool.cache.BaseAmount
}
func (pool *singlePool) quoteDepth() Uint {
	return pool.cache.QuoteAmount
}
func (pool *singlePool) getPrice() Uint {
	return pool.quoteDepth().Quo(pool.baseDepth())
}
func (pool *singlePool) getFee(swap SwapTo) Uint {
	if pool.flipped {
		if swap == SwapToQuote {
			return OneUint().Quo(pool.getPrice())
		} else {
			return 1 // 1 RUNE
		}
	} else {
		if swap == SwapToAsset {
			return OneUint().Quo(pool.getPrice()) // 1 RUNE in Asset
		} else {
			return 1 // 1 RUNE
		}
	}
}
func (pool *singlePool) complete() bool {
	return pool.baseDepth().GT(ZeroUint()) && pool.quoteDepth().GT(ZeroUint())
}
func NewPool(pool1, pool2 *Pool) *Pool {
	if 	pool1.IsDual() || pool2.IsDual() ||
		pool1.exchange != pool2.exchange || 
		!pool1.market.QuoteAsset.Equal(pool2.market.QuoteAsset) || 
		pool1.IsFlipped() || pool2.IsFlipped() {
		panic("cannot create DualPool off these pools")
	}
	pool := &Pool{
		exchange:	pool1.exchange,
		market:		NewMarket(pool1.market.BaseAsset, pool2.market.BaseAsset),
		pools:		[]singlePool{ pool1.pools[0], pool2.pools[0] },
	}
	return pool
}
func (pool *Pool) UpdateCache() {
	pool.pools[0].UpdateCache()
	if pool.IsDual() {
		pool.pools[1].UpdateCache()
	}
}
func (pool *Pool) IsSingle() bool {
	return len(pool.pools) == 1
}
func (pool *Pool) IsDual() bool {
	return len(pool.pools) == 2
}
func (pool *Pool) IsFlipped() bool {
	if pool.IsDual() { return false }
	return pool.pools[0].flipped
}
func (pool *Pool) base() Asset {
	return pool.pools[0].base()
}
func (pool *Pool) quote() Asset {
	if pool.IsSingle() { 
		return pool.pools[0].quote() 
	} else { // double
		return pool.pools[1].base() 
	}
}
// Now someone swaps 250 RUNE (x) into the pool for  16 USD (y).
// (x*X*Y)/(x+X)^2 ie. (250*1000*100)/(250+1000)^2
// GetSwapReturn(x - amount, swap - if SELL, swap to RUNE, if BUY, swap to Asset)
func (pool *Pool) GetSwapReturn(x Uint, swap SwapTo) (output Uint) {
	get := func(x Uint, single singlePool, swap SwapTo) Uint {
		// output = (x*X*Y)/(x+X)^2
		X := single.quoteDepth()
		Y := single.baseDepth()
		if swap == SwapToQuote {X, Y = Y, X}
		numerator := x.Mul(X).Mul(Y) //x * X * Y 
		denominator := (x.Add(X)).Mul(x.Add(X)) // * (x + X)
		return numerator.Quo(denominator)
	}
	var fee Uint
	if pool.IsSingle() {
		output = get(x, pool.pools[0], swap)
		fee = pool.pools[0].getFee(swap)
	} else { // DualPool
		if swap == SwapToAsset {
			output = get(x, pool.pools[1], SwapToQuote)
			fee = pool.pools[1].getFee(SwapToQuote)
			output = get(output, pool.pools[0], SwapToAsset)
			fee = fee.Quo(pool.pools[0].getPrice()) + pool.pools[0].getFee(SwapToAsset)
		} else { // SwapToQuote = swap from first pool asset to second pool asset
			output = get(x, pool.pools[0], SwapToQuote)
			fee = pool.pools[0].getFee(SwapToQuote)
			output = get(output, pool.pools[1], SwapToAsset)
			fee = fee.Quo(pool.pools[1].getPrice()) + pool.pools[1].getFee(SwapToAsset)
		}
	}
	output = output.Sub(fee) // 1 or 2 RUNE - network fee
	return output
}
func (pool *Pool) GetMarket() Market {
	return pool.market
}
func (pool *Pool) GetInnerMarket(i int) Market {
	if pool.IsFlipped() {
		return NewMarket(pool.pools[i].market.QuoteAsset, pool.pools[i].market.BaseAsset)
	} else {
		return pool.pools[i].market
	}
}
func (pool *Pool) GetOrderbook() *Orderbook {
	panic("not supported")
}
func (pool *Pool) AtomicSetInnerData(i int, depths Amounts) {
	pool.pools[i].atomicSetData(depths)
}
func (pool *Pool) GetPool() *Pool {
	return pool
}
func (pool *Pool) baseDepth(i int) Uint {
	return pool.pools[i].baseDepth()
}
func (pool *Pool) quoteDepth(i int) Uint {
	return pool.pools[i].quoteDepth()
}
func (pool *Pool) GetQuoteDepth() Uint {
	if pool.IsSingle() {
		return pool.quoteDepth(0)
	} else { // DuaPool
		if pool.quoteDepth(0).LTE(pool.quoteDepth(1)) { // if pool1.rune < pool2.rune, return pool1.asset
			k := pool.quoteDepth(0).Quo(pool.quoteDepth(1))
			return k.Mul(pool.baseDepth(1))
		} else { 
			return pool.baseDepth(1)
		}
	}
}
func (pool *Pool) GetBaseDepth() Uint {
	if len(pool.pools) == 1 {
		return pool.baseDepth(0)
	} else { // DuaPool
		if pool.quoteDepth(0).LTE(pool.quoteDepth(1)) { // if pool1.rune < pool2.rune, return pool1.asset
			return pool.baseDepth(0)
		} else { 
			k := pool.quoteDepth(1).Quo(pool.quoteDepth(0))
			return k.Mul(pool.baseDepth(0))
		}
	}
}
func (pool *Pool) GetPrice() Uint {
	if pool.IsSingle() {
		if pool.IsEmpty() { return ZeroUint() }
		return pool.pools[0].getPrice()
	} else { // DuaPool
		return pool.pools[0].getPrice().Quo(pool.pools[1].getPrice())
	}
}
func (pool *Pool) GetPriceInRune() Uint {
	if pool.IsFlipped() { 
		return OneUint()
	} else {
		return pool.pools[0].getPrice()
	}
}
func (pool *Pool) IsEmpty() bool {
	return pool.pools[0].cache.IsEmpty() || len(pool.pools) == 2 && pool.pools[1].cache.IsEmpty()
}
func (pool *Pool) Equal(pool2 *Pool) bool {
	if !pool.market.Equal(pool2.market) || len(pool.pools) != len(pool2.pools) || !pool.pools[0].cache.Equal(pool2.pools[0].cache) {
		return false
	}
	if pool.IsDual() && !pool.pools[1].cache.Equal(pool2.pools[1].cache)  {
		return false
	}
	return true
}
func (pool *Pool) Merge(Offer) (Offer, error) {
	panic("PoolData.Merge not implemented")
}
func (pool *Pool) GetTradeSize(price Uint, maxam Uint) (amount Uint, profit Uint) {
	if pool.IsSingle() {
		amount, profit = pool.pools[0].getTradeSize(price, maxam)
	} else {
		amount, profit = pool.dualPoolGetTradeSize(price, maxam)
	}
	return amount, profit
}
func (single *singlePool) getTradeSize(price Uint, maxam Uint) (amount Uint, profit Uint) {
	X := single.quoteDepth() // ASSET, if flipped then RUNE
	Y := single.baseDepth() // RUNE, if flipped then ASSET
	d1 := price
	d2 := X.Quo(Y) // X/Y
	var d Uint
	if d1.GT(d2) {
		d = d1.Sub(d2).Quo(d1)
	} else {
		d = d2.Sub(d1).Quo(d2)
	}
	amount = d.Mul(X).QuoUint64(5) // d * X / 5
	if amount.GT(maxam) {
		amount = maxam
	}
	profit = d.Mul(amount).Quo(d2) // d * amount / d2
	return amount, profit
}
func (pool *Pool) dualPoolGetTradeSize(price Uint, maxam Uint) (amount Uint, profit Uint) {
	assetpool := pool.pools[0]
	basepool := pool.pools[1]
	// let minRune = min(bnb.rune/bolt.rune,1) (this is to factor in liquidity in the bnb pool)
	// BNB trade size is calculated by  (d * m * R * X)/(5* Y) or (% difference * minRune * bolt.rune * bnb.bnb)/(5 * bnb.rune) (Depths)
	R_asset_rune_depth := assetpool.quoteDepth()
	a_asset_depth := assetpool.baseDepth()
	Y_quote_rune_depth := basepool.quoteDepth()
	X_quote_depth := basepool.baseDepth()
	R := R_asset_rune_depth
	a := a_asset_depth
	X := X_quote_depth
	Y := Y_quote_rune_depth
	// new alg
	m := OneUint()
	if R.GT(Y) { m = Y.Quo(R) } // take into account second (eg. BNB) pool depth
	assetPriceInRune := R.Quo(a) // R_asset_rune_depth / a_asset_depth = 10 / 20 = 0.5
	basePriceInRune := Y.Quo(X)
	d1 := price
	d2 := assetPriceInRune.Quo(basePriceInRune)
	var d Uint
	if d1.GT(d2) {
		d = d1.Sub(d2).Quo(d1)
	} else {
		d = d2.Sub(d1).Quo(d2)
	}
	amount = d.Mul(m).Mul(a).QuoUint64(5)
	if amount.GT(maxam) {
		amount = maxam
	}
	profit = d.Mul(amount).Mul(assetPriceInRune) // Y / X - basePriceInRune

	return amount, profit
}
func (pool *Pool) String() string {
	if len(pool.pools) == 1 {
		return fmt.Sprintf("pool [%s]:[b: %s / q: %s @ %s]", pool.market, pool.baseDepth(0), pool.quoteDepth(0), pool.GetPrice())
	} else {
		return fmt.Sprintf("dual pool [%s]: POOL1: {[%s]:[b: %s / q: %s] @ %s} POOL2: [%s]:[b: %s / q: %s @ %s] @ PRICE: %s", pool.market, pool.pools[0].market, pool.baseDepth(0), pool.quoteDepth(0), pool.pools[0].getPrice(), pool.pools[1].market, pool.baseDepth(1), pool.quoteDepth(1), pool.pools[1].getPrice(), pool.GetPrice())
	}
}
func (pool *Pool) Refresh() *Pool {
	var flipMarket Market
	if pool.IsFlipped() {
		flipMarket = NewMarket(pool.market.QuoteAsset, pool.market.BaseAsset)
	} else {
		flipMarket = pool.market
	}
	depths := pool.exchange.GetCurrentOfferData(flipMarket).(Amounts)
	if !depths.IsEmpty() {
		if pool.IsFlipped() { depths.Flip() }
		pool.pools[0].cache = depths
	}
	if pool.IsDual() {
		depths := pool.exchange.GetCurrentOfferData(pool.pools[1].market).(Amounts)
		if !depths.IsEmpty() {
			pool.pools[1].atomicSetData(depths)
			pool.pools[1].cache = depths
		}
	}
	return pool
}
func (pool *Pool) GetExchange() Exchange {
	return pool.exchange
}
func (pool *Pool) Complete() bool {
	if !pool.pools[0].complete() {
		return false
	}
	if pool.IsDual() && !pool.pools[1].complete() {
		return false
	}
	return true
}
func (pool *Pool) GetSwapper() Swapper {
	return pool.exchange.GetSwapper(pool.market)
}
func pool_interface_test() {
	var _ Offer = &Pool{}
}