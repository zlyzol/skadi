package bot

import (
	"fmt"
	"os"
	"time"
	"math"
	"github.com/rs/zerolog"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
)
func (h *Hunter) getAccLimits(side common.OrderSide) (limits, accLimits common.Amounts) {
	a, q := h.ob.GetMarket().GetAssets()
	bal := func(off common.Offer, a common.Asset) common.Uint{ return off.GetExchange().GetAccount().GetBalance(a) }
	var abal, qbal common.Uint
	sell := side == common.OrderSideSELL
	if sell {
		abal = bal(h.ob, a)
		qbal = bal(h.pool, q)
	} else { // BUY
		abal = bal(h.pool, a)
		qbal = bal(h.ob, q)
	}
	accLimits = common.Amounts{ BaseAmount: abal, QuoteAmount: qbal }
	accLimits = h.ob.GetExchange().UpdateLimits(accLimits, h.ob.GetMarket(), h.pool.GetExchange(), side) // at the moment this does just 99% 
	iff := func(c bool, v1, v2 common.Uint) common.Uint { if c { return v1 } else { return v2 }}
	limits = common.Amounts{ 
		BaseAmount: iff(sell, abal, common.MaxUintValue),
		QuoteAmount: iff(sell, common.MaxUintValue, qbal),
	}
	return limits, accLimits
}
/*
func (h *Hunter) getPoolPrice(pool common.Pool) common.Uint { 
	pr := pool.GetPrice()
	if h.flipPool {
		return common.OneUint().Quo(pr)
	} else {
		return pr
	}
}
*/
func (h *Hunter) findPrey() *Prey {
	var side common.OrderSide // BUY or SELL
	var diff float64
	var ba common.OrderbookEntries
	bids, asks := h.ob.GetEntries() 
	if len(bids) == 0 || len(asks) == 0 || !h.pool.Complete() {
		h.logger.Info().Int("len(bids)", len(bids)).Int("len(asks)", len(asks)).Bool("pool complete", h.pool.Complete()).Msg("ob/pool not prepared yet")
		return nil
	}
	
	if Debug_do {
		if h.ob.GetExchange().GetName() == Debug_ex { 
			if h.market.BaseAsset.Ticker.String() == Debug_bt {
				if h.market.QuoteAsset.Ticker.String() == Debug_qt {
					return NewPrey(Debug_prey.side, Debug_prey.amount, Debug_prey.limBuyPrice, Debug_prey.limBuyPrice, 1, h)
				}
			}
		} 
		return nil
	}

	bidpr, askpr := bids[0].Price, asks[0].Price
	poolpr := h.pool.GetPrice()
	if len(bids) > 0 && poolpr.LT(bidpr) { // the pool price is less than bid in orderbook -> sell on ex
		ba = bids
		side = common.OrderSideSELL
		diff = bidpr.Sub(poolpr).MulUint64(100).Quo(bidpr).ToFloat()
	} else if len(asks) > 0 && poolpr.GT(askpr) { // the pool price is greater than ask in orderbook -> buy on ex
		ba = asks
		side = common.OrderSideBUY
		diff = poolpr.Sub(bidpr).MulUint64(100).Quo(poolpr).ToFloat()
	} else { // there is no arbitrage opportunity
		debug_secs := time.Now().Sub(h.lastCheck).Seconds()
		if debug_secs /*time.Now().Sub(h.lastCheck).Seconds()*/ >= 10 {
			h.logger.Info().Msgf("No prey found (%s / %s // %s)", h.ob.GetEntry(BID, 0), h.ob.GetEntry(ASK, 0), h.pool.GetPrice())
			h.lastCheck = time.Now()
		}
		h.logger.Debug().Msgf("No prey found (%s / %s // %s)", h.ob.GetEntry(BID, 0), h.ob.GetEntry(ASK, 0), h.pool.GetPrice())
		return nil
	}
	if err := ba.RepareConsistency(); err != nil {
		h.logger.Error().Err(err)
		return nil
	}
	if diff < c.MIN_PERC_DIFF || diff > c.MAX_PERC_DIFF {
		h.logger.Info().Float64("diff %", diff).Str("ask", h.ob.GetEntry(ASK, 0).String()).Str("bid", h.ob.GetEntry(BID, 0).String()).Str("pool", h.pool.GetPrice().String()).Msg("Price diff % out of range")
		return nil
	}
	limits, _ := h.getAccLimits(side) // account funds limits
	limits = h.updateGlobalLimits(limits)
	am, runeAm, avg, yield := h.calcArbRes(ba, poolpr, side, h.pool, limits, func() *zerolog.Event { return h.logger.Debug() })
	if runeAm.GTE(c.MIN_RUNE_ARB_AMOUNT) && yield.GTE(common.NewUintFromFloat(c.MIN_RUNE_YIELD)) { // everything ok, no brainer - do arbitrage
		lim := ba.LimitPriceForAmount(am)

		slow := true

		if slow {
			///*
				// debug calcArbRes & checkPool
				_, _, _, _ = h.calcArbRes(ba, poolpr, side, h.pool, limits, func() *zerolog.Event { return h.logger.Info().Str("huhu", "huhu") })
			
			change := h.dbg_doubleCheckPool(side, poolpr, am, avg)
			if change > 0.005 { 
				h.logger.Info().Msgf("change is too much %.3f", change)
				return nil 
			}
			//*/
			
			h.logger.Info().Msgf("Debug pool info 1 (%s / %s // %s)", h.ob.GetEntry(BID, 0), h.ob.GetEntry(ASK, 0), h.pool)
		}
		return NewPrey(side, am, lim, avg, yield, h)
	} else {
		h.logger.Debug().Msgf("Debug pool info 1 (%s / %s // %s)", h.ob.GetEntry(BID, 0), h.ob.GetEntry(ASK, 0), h.pool)
		h.logger.Info().Msgf("diff is small (rune Am: %s, yield: %s)", runeAm, yield)
		if runeAm.IsZero() {
			am1, runeAm1, avg1, yield1 := h.calcArbRes(ba, poolpr, side, h.pool, limits, func() *zerolog.Event { return h.logger.Debug() })
			_, _, _, _ = am1, runeAm1, avg1, yield1 
		}
	}
	h.debug_noPreyCheck(ba, side, am, runeAm, yield)
	//time.Sleep(c.WAIT_AFTER_ZERO_WALLET_SEC * time.Second)
	return nil
}
func (h *Hunter) dbg_doubleCheckPool(side common.OrderSide, poolpr, am, avg common.Uint) (change float64) {
	h.pool.Refresh()
	newPoolpr := h.pool.GetPrice()
	if !newPoolpr.Equal(poolpr) {
		change = math.Abs(poolpr.ToFloat() - newPoolpr.ToFloat()) / math.Max(poolpr.ToFloat(), newPoolpr.ToFloat())
		if 	side == common.OrderSideBUY && newPoolpr.GT(poolpr) || // we buy on ex and sell on tc -> if tc price is higher thats ok
			side == common.OrderSideSELL && newPoolpr.LT(poolpr) {  // we sell on ex and buy on tc -> if tc price is lower thats ok
				h.logger.Info().Str("old price", poolpr.String()).Str("new price", newPoolpr.String()).Float64("change %", change).Msg("meanwhile pools changed in good direction -> its ok, continue")
				h.logger.Info().Msgf("meanwhile pools changed in good direction -> its ok, continue (old->new/diff: %s -> %s / %.4f", poolpr, newPoolpr, change)
			change = 0 // we dot mind the change it is in our favor
		} else {
			h.logger.Info().Msgf("meanwhile pools changed in good direction (old->new/diff: %s -> %s / %.4f", poolpr, newPoolpr, change)
		}
		_, s := common.YieldInRune_debug(am, avg, side, h.pool)
		h.logger.Info().Msgf("NEW YIELD CALC %s (poolpr = %s)", s, newPoolpr)
	}
	return change
}
func (h *Hunter) updateGlobalLimits(limits common.Amounts) common.Amounts {
	maxRune := common.NewUint(c.MAX_ARB_RUNE)
	price := common.Oracle.GetRunePriceOf(h.market.BaseAsset)
	if price.Mul(limits.BaseAmount).GT(maxRune) { //2 * 300 > 200 -> 600/200=3 ... 300/3 /// pr * am > max -> am*max / pr*am
		h.logger.Debug().Str("amount", limits.BaseAmount.String()).Str("shrinked to", limits.BaseAmount.Mul(maxRune).Quo(price.Mul(limits.BaseAmount)).String()).Msg("asset limits shrinked due to maxRune constant")
		limits.BaseAmount = limits.BaseAmount.Mul(maxRune).Quo(price.Mul(limits.BaseAmount))
	}
	price = common.Oracle.GetRunePriceOf(h.market.QuoteAsset)
	if price.Mul(limits.QuoteAmount).GT(maxRune) { //2 * 300 > 200 -> 600/200=3 ... 300/3 /// pr * am > max -> am*max / pr*am
		h.logger.Debug().Str("amount", limits.BaseAmount.String()).Str("shrinked to", limits.QuoteAmount.Mul(maxRune).Quo(price.Mul(limits.QuoteAmount)).String()).Msg("quoted limits shrinked due to maxRune constant")
		limits.QuoteAmount = limits.QuoteAmount.Mul(maxRune).Quo(price.Mul(limits.QuoteAmount))
	}
	return limits
}
func (h *Hunter) calcArbRes(ba common.OrderbookEntries, poolpr common.Uint, side common.OrderSide, pool *common.Pool, limits common.Amounts, loggerf func() *zerolog.Event) (am, runeAm, avg, yield common.Uint) {
	loggerf().Msgf("calcArbRes - pool: %s", h.pool)
	limam := common.MinUint(limits.BaseAmount, limits.QuoteAmount.Quo(poolpr))
	maxam := ba.AmountForPrice(poolpr, side) // maximum amount in asset for trade for the current pool price
	maxam = common.MinUint(maxam, limam)
	am = maxam // current amount in asset
	var min, max common.Uint = common.ZeroUint(), common.MaxUintValue
	var cycles bool = false
	var maxLoop int = 100
	for {
		pr, oecnt := ba.Avg(am)	// average price (asset price in rune) for all OEs up to am
		poolam, profit := pool.GetTradeSize(pr, maxam) // in poolam in asset, profit in rune
		var delta float64 = 0.0
		if oecnt == 1 && am.GTE(poolam) { // we use only 1 OB entry and we already have bigger amount from OB than from pool => use straight the pool amount
			am = poolam
		} else {
			amG, amL := am, poolam
			if amG.LT(amL) { amG, amL = amL, amG }
			if amL.LTE(min) || amG.GTE(max) {
				cycles = true
				am = amL
			}
			min, max = common.MaxUint(min, amL), common.MinUint(max, amG)
			delta = amG.Sub(amL).Quo(amL).MulUint64(100).ToFloat() // delta in %
		}
		loggerf().
			Int("count", oecnt).
			Str("am", am.String()).
			Str("poolam", poolam.String()).
			Str("profit", profit.String()).
			Str("pool price", poolpr.String()).
			Bool("cycles", cycles).
			Float64("delta (max 2%)", delta).
			Msg("trying to find optimal trade size")
		maxLoop--
		if cycles || delta < 2 || maxLoop <= 0 { // if amount difference between OB price and pool price is less then 2% then it is OK, go to arb
			am = common.MinUint(am, poolam)
			loggerf().Int("count", oecnt).Msg("found best tradeSize")
			break
		}
		am = am.Add(poolam).QuoUint64(2)
	}
	if am.IsZero() {
		loggerf().Msg("amount for arb is zero")
		return common.ZeroUint(), common.ZeroUint(), common.ZeroUint(), common.ZeroUint()
	}
	runeAm = am.Mul(pool.GetPriceInRune())
	avg, _ = ba.Avg(am)
	if avg.IsZero() {
		loggerf().Msg("avg is zero")
		avg, _ = ba.Avg(am)
		return common.ZeroUint(), common.ZeroUint(), common.ZeroUint(), common.ZeroUint()
	}
	yield = common.YieldInRune(am, avg, side, pool, loggerf)
	return am, runeAm, avg, yield
}
func (h *Hunter) debug_noPreyCheck(ba common.OrderbookEntries, side common.OrderSide, am, runeAm, yield common.Uint) {
	poolpr := h.pool.GetPrice()
	// without any limit (big account, no global limit set) - no account try without limits to see if account amounts are the limiting factor
	nolimits := common.Amounts{ BaseAmount: common.MaxUintValue, QuoteAmount: common.MaxUintValue }
	amNoLimits, runeAmNoLimits, _, yieldNoLimits := h.calcArbRes(ba, poolpr, side, h.pool, nolimits, func() *zerolog.Event { return h.logger.Debug() })

	// just with wallet amount limit (current account, no global limit set) - no account try without limits to see if account amounts are the limiting factor
	limits, _ := h.getAccLimits(side) // account funds limits
	amAccLimits, runeAmAccLimits, _, yieldAccLimits := h.calcArbRes(ba, poolpr, side, h.pool, limits, func() *zerolog.Event { return h.logger.Debug() })

	// just global amount limit (no account limit set)
	limits = h.updateGlobalLimits(nolimits) // global limits
	amGlbLimits, runeAmGlbLimits, _, yieldGlbLimits := h.calcArbRes(ba, poolpr, side, h.pool, limits, func() *zerolog.Event { return h.logger.Debug() })

	// if this is true, no limit yeald is too small, we can't do anything with this
	if yieldNoLimits.LT(common.NewUintFromFloat(c.MIN_RUNE_YIELD)) || runeAmNoLimits.LT(c.MIN_RUNE_ARB_AMOUNT) {
		h.logger.Debug().Str("yield", yield.String()).Msg("yield too small")
		return
	}
	// if we are here, some limit is limiting us
	
	// yield is small just because of wallet amount (or global rune trade limit)
	debug_secs := time.Now().Sub(h.lastCheck).Seconds()
	if debug_secs >= 30 {
		var amLimits, runeAmLimits, yieldLimits common.Uint
		var whatLimits string
		if runeAmAccLimits > runeAmGlbLimits {
			amLimits, runeAmLimits, yieldLimits = amGlbLimits, runeAmGlbLimits, yieldGlbLimits
			whatLimits = "global"
		} else {
			amLimits, runeAmLimits, yieldLimits = amAccLimits, runeAmAccLimits, yieldAccLimits
			whatLimits = "wallet"
		}
		if yieldNoLimits.GTE(common.NewUintFromFloat(c.MIN_RUNE_YIELD)) && amNoLimits.GTE(c.MIN_RUNE_ARB_AMOUNT) {
			s := fmt.Sprintf("%s limit stop (no arb): %s: %s->%s (=%s) (in rune: %s->%s (=%s)) yield: %s->%s (=%s)", whatLimits, h.market, amNoLimits, am, amLimits,
			runeAmNoLimits, runeAm, runeAmLimits, yieldNoLimits, yield, yieldLimits)
			h.logger.Info().Msg(s)
			debug_saveInfo(s)
			debug_s1 := fmt.Sprintf("%s", amLimits)
			debug_s2 := fmt.Sprintf("%s", am)
			if debug_s1 != debug_s2 {
				h.logger.Info().Msg("Strange, amounts should be equal")
			}
			return
		}
		base := h.ob.GetMarket().BaseAsset
		quote := h.ob.GetMarket().QuoteAsset
		qam, qamNoLimits := am.Mul(poolpr), amNoLimits.Mul(poolpr)	
		var exo common.Offer
		var t string
		var am1, am2 common.Uint	
		if side == common.OrderSideSELL {
			if amNoLimits.GT(h.ob.GetExchange().GetAccount().GetBalance(h.ob.GetMarket().BaseAsset)) {
				//h.logger.Info().Msgf("not enough %s on exchange, have [%s] / need [%s]", base.Ticker, am, amNoLimits)
				exo = h.ob; t = base.Ticker.String(); am1 = am; am2 = amNoLimits
			} else {
				//h.logger.Info().Msgf("not enough %s on dex wallet, have [%s] / need [%s]", quote.Ticker, qam, qamNoLimits)
				exo = h.pool; t = quote.Ticker.String(); am1 = qam; am2 = qamNoLimits
			}
		} else { // BUY
			if amNoLimits.GT(h.ob.GetExchange().GetAccount().GetBalance(h.ob.GetMarket().BaseAsset)) {
				//h.logger.Info().Msgf("not enough %s on exchange, have [%s] / need [%s]", quote.Ticker, qam, qamNoLimits)
				exo = h.ob; t = quote.Ticker.String(); am1 = qam; am2 = qamNoLimits
			} else {
				//h.logger.Info().Msgf("not enough %s on dex wallet, have [%s] / need [%s]", base.Ticker, am, amNoLimits)
				exo = h.pool; t = base.Ticker.String(); am1 = am; am2 = amNoLimits
			}
		}
		h.logger.Info().Msgf("not enough %s on %s, have [%s] / need [%s]", t, exo.GetExchange().GetName(), am1, am2)
		debug_saveInfo(fmt.Sprintf("small account stop (%s): not enough %s on %s, have [%s] / need [%s]", h.market, t, exo.GetExchange().GetName(), am1, am2))
		h.logger.Debug().Str("yield / yieldNoLimits", yield.String() + " / " + yieldNoLimits.String()).Msg("yield too small but could be OK")
		h.lastYieldInfo = time.Now()
	}
}

func debug_saveInfo(s string) {
	f, err := os.OpenFile("info.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		now := time.Now(); nows := fmt.Sprintf("%d.%02d %d:%02d:%02d", now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
		s := fmt.Sprintf("%s: %s\n", nows, s)
		f.WriteString(s)
		f.Close()
	}
}