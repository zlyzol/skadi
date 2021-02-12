package common

import (
	"fmt"
	"math"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func YieldInRune(am Uint, avg Uint, trade OrderSide, pool *Pool, loggerfold func() *zerolog.Event) Uint {
	loggerf := func() *zerolog.Event { return loggerfold().Str("module", "EstimatedYield") }
	res := yieldInAsset(am, avg, trade, pool, loggerf) // yield in asset
	res = res.Mul(pool.GetPriceInRune())
	return res
}
func yieldInAsset(am Uint, avg Uint, trade OrderSide, pool *Pool, loggerf func() *zerolog.Event) Uint {
	sellSwapTo, buySwapTo := SwapToAsset, SwapToQuote
	srcTicker, dstTicker := pool.GetMarket().BaseAsset.Ticker.String(), pool.GetMarket().QuoteAsset.Ticker.String()
	var swap, sell, buy, get, res Uint
	if trade == OrderSideSELL { // sell asset on ex and swap quote for asset
		sell = am
		swap = sell.Mul(avg)                           // in quote
		get = pool.GetSwapReturn(swap, sellSwapTo)	 // normal - result in asset
		res = get.Sub(sell)                            // normal - result in asset
		loggerf().Msg(fmt.Sprintf("EstimatedYield sell %s %s for %s %s -> swap them for %s %s, plus = %s RUNE (%s %s)", sell, srcTicker, swap, dstTicker, get, srcTicker, res.Mul(pool.GetPriceInRune()), res, srcTicker))
	} else { // buy - buy asset on ex and swap asset for quote
		swap = am                                   // in asset
		buy = pool.GetSwapReturn(swap, buySwapTo)	// normal - result in quote
		get = buy.Quo(avg)                          // normal - result in asset
		res = get.Sub(swap)                         // normal - result in asset
		loggerf().Msg(fmt.Sprintf("EstimatedYield buy %s %s for %s %s -> use them to buy %s %s, plus = %s RUNE, %s %s", swap, srcTicker, buy, dstTicker, get, srcTicker, res.Mul(pool.GetPriceInRune()), res, srcTicker))
	} // buy 80 coty (got 0.136 bnb) -> swap 80 coty for bnb (got 0.113 bnb)
	  // swap 80 coty (got 0.113 bnb) -> use all bnb and buy coti 
	if math.IsNaN(float64(res)) {
		loggerf().Msg("res is NaN")
		panic("")
	}
	return res
}
func YieldInRune_debug(am Uint, avg Uint, trade OrderSide, pool *Pool) (Uint, string) {
	res, s := yieldInAsset_debug(am, avg, trade, pool) // yield in asset
	if !pool.IsFlipped() {
		apr := pool.GetPriceInRune()
		res = res.Mul(apr)
	}
	return res, s
}
func yieldInAsset_debug(am Uint, avg Uint, trade OrderSide, pool *Pool) (Uint, string) {
	var s string
	sellSwapTo, buySwapTo := SwapToAsset, SwapToQuote
	srcTicker, dstTicker := pool.GetMarket().BaseAsset.Ticker.String(), pool.GetMarket().QuoteAsset.Ticker.String()
	logger := log.With().Str("module", "EstimatedYield").Logger()
	var swap, sell, buy, got, yield Uint
	if trade == OrderSideSELL { // sell asset on ex and swap quote for asset
		sell = am
		swap = sell.Mul(avg)                           // in quote
		got = pool.GetSwapReturn(swap, sellSwapTo)	 // normal - result in asset, flip - result in quote
		yield = got.Sub(am)                            // normal - result in asset, flip - result in quote
		s = fmt.Sprintf("EstimatedYield sell %s %s @ %s = %s %s -> swap back for %s %s @ %s, yield = %s", sell, srcTicker, avg, swap, dstTicker, got, srcTicker, swap.Quo(got), yield)
	} else { // buy - swap asset for quote and buy asset on ex
		swap = am                                   // in asset
		buy = pool.GetSwapReturn(swap, buySwapTo)	// normal - result in quote, flip - result in asset
		got = buy.Quo(avg)                          // normal - result in asset, flip - result in asset
		yield = got.Sub(swap)                         // normal - result in asset, flip - result in asset
		s = fmt.Sprintf("EstimatedYield swap %s %s -> %s %s @ %s -> buy %s %s @ %s = %s %s, yield = %s", swap, srcTicker, buy,dstTicker , swap.Quo(buy), got, srcTicker, avg, buy, dstTicker, yield)
	}
	if math.IsNaN(float64(yield)) {
		logger.Debug().Msg(fmt.Sprintf("NaN error - EstimatedYield %s sell - sell = %s RUNE for %s %s -> swap them for %s RUNE, result = %s", srcTicker, sell, dstTicker, swap, got, yield))
		panic("")
	}
	return yield, s
}
