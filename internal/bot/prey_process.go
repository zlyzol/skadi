package bot

import (

	//	"github.com/ethereum/go-ethereum/p2p"
	//	"github.com/pkg/errors"

	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	//	"gitlab.com/zlyzol/skadi/internal/exchange"
)

var BID, ASK = common.OfferSide.BID, common.OfferSide.ASK
var OB, POOL = common.OfferType.ORDERBOOK, common.OfferType.TCPOOL
var BUY, SELL = common.OrderSideBUY, common.OrderSideSELL

// Prey structure
type Prey struct {
	logger       zerolog.Logger
	side		 common.OrderSide
	amount       common.Uint
	limBuyPrice  common.Uint
	limSellPrice common.Uint
	h		 	 *Hunter
	expectYield	 common.Uint
}

func NewPrey(side common.OrderSide, amount, lim, avg, expectYield common.Uint, h *Hunter) *Prey {
	prey := Prey{
		logger:		  	log.With().Str("module", "prey").Str("exchange", h.ob.GetExchange().GetName()).Str("market", h.ob.GetMarket().String()).Logger(),
		side:        	side,
		amount:       	amount,
		limBuyPrice:  	side.IfBuy(lim, avg),
		limSellPrice: 	side.IfBuy(avg, lim),
		h:				h,
		expectYield:	expectYield,
	}
	if lim.IsZero() || avg.IsZero() {
		prey.logger.Error().Msg("lim or avg is zero")
		panic("lim or avg is zero")
	}
	return &prey
}

type MockSwapper struct { pool *common.Pool }
func (mt MockSwapper) Swap(side common.SwapTo, amount, limit common.Uint) common.Order { 
	oside := common.OrderSideSELL
	if side == common.SwapToQuote { oside = common.OrderSideBUY }
	estimated := mt.pool.GetSwapReturn(amount, side)
	return MockOrder{side: oside, amount: estimated, limit: limit} 
}
type MockTrader struct {}
func (mt MockTrader) Trade(side common.OrderSide, amount, limit common.Uint) common.Order { return MockOrder{side: side, amount: amount, limit: limit} }
type MockOrder struct {side common.OrderSide; amount, limit common.Uint}
func (mo MockOrder) GetResult() common.Result {
		muldiv := mo.amount.Mul
		if mo.side == common.OrderSideBUY { muldiv = mo.amount.Quo }
		return common.Result {
			Err: nil,
			PartialFill: false,
    		Amount: mo.amount,
  			QuoteAmount: muldiv(mo.limit),
    		AvgPrice: mo.limit,
	}
}
func (mo MockOrder) Revert() error { return nil }
func (mo MockOrder) PartialRevert(amount common.Uint) error { return nil }

func (prey *Prey) process() {
	pr := newPreyResult(prey)
	trader := prey.h.ob.GetTrader()
	swapper := prey.h.pool.GetSwapper()

	if Debug_do {
		if Debug_trader { trader = MockTrader{} }
		if Debug_swapper { swapper = MockSwapper{pool: prey.h.pool} }
	}
	var order, swap common.Order
	order = trader.Trade(prey.side, prey.amount, prey.limSellPrice)
	time.Sleep(400 * time.Millisecond) // wait for swap result arrival into accont
	ores := order.GetResult()
	if ores.Err != nil {
		prey.logger.Error().Err(ores.Err).Msg("prey 1-st order failed -> return")
		pr.printEndLog(prey.h.store)
		return
	}
	//flip := prey.h.pool.IsFlipped()
	side, amount := getSideAmount(prey.side, ores.Amount, ores.QuoteAmount)
	var runeAm common.Uint
	/*if flip {
		runeAm = common.Oracle.GetRuneValueOf(amount, side.Invert().SrcAsset(prey.h.ob.GetMarket()))
	} else {*/
		runeAm = common.Oracle.GetRuneValueOf(amount, side.SrcAsset(prey.h.ob.GetMarket()))
	//}
	if runeAm < 5 {
		err := errors.Errorf("1. order result amount in RUNE is less than 5 RUNE -> cancelling arb (bought/sold asset remains as is)")
		prey.logger.Error().Err(err).Msg("cancel")
		return
	}
	var alreadyBalanced uint8
	if !prey.enoughFunds(side, amount) {
		err := prey.balanceFirstAccounts(&ores)
		_, amount = getSideAmount(prey.side, ores.Amount, ores.QuoteAmount)
		if err != nil {
			err := order.Revert()
			if err != nil {
				prey.logger.Error().Err(err).Msg("prey 1-st order revert failed")
			}
			prey.logger.Error().Err(err).Msg("****************BALANCE ERROR - BALANCE 1. - SHOULD NOT HAPPEN")
			pr.printEndLog(prey.h.store)
			return	
		}
		alreadyBalanced = 1
	}

	// debug estimate 2
	estimated := prey.h.pool.GetSwapReturn(amount, side)
	prey.logger.Info().Str("estimated", estimated.String()).Msg("estimated swap return 2")

	/*
	// debug pool info 2
	iff := func(c bool, v1, v2 common.Uint) common.Uint { if c { return v1 } else { return v2 }}
	pooo := prey.h.pool
	pooopr := pooo.GetPrice()
	change1 := prey.h.dbg_doubleCheckPool(prey.side, pooopr, prey.amount, iff(prey.side == common.OrderSideBUY, prey.limBuyPrice, prey.limSellPrice))
	prey.logger.Info().Msgf("Debug pool info 2 %s", prey.h.pool.Refresh())
	*/

	// THE SWAP
	swap = swapper.Swap(side, amount, common.ZeroUint())

	/*
	// debug pool info 3
	prey.logger.Info().Msgf("Debug pool info 3 %s", prey.h.pool.Refresh())
	pooo = prey.h.pool
	pooopr = pooo.GetPrice()
	change2 := prey.h.dbg_doubleCheckPool(prey.side, pooopr, prey.amount, iff(prey.side == common.OrderSideBUY, prey.limBuyPrice, prey.limSellPrice))
	prey.logger.Info().Msgf("pool price changes %.4f / %.4f", change1, change2)
	*/

	// continue with swap result processing
	sres := swap.GetResult()
	if sres.Err != nil {
		prey.logger.Error().Err(sres.Err).Msg("****************BALANCE ERROR - SWAP - SHOULD NOT HAPPEN")
		prey.logger.Error().Err(sres.Err).Msg("prey 2-nd order failed -> revert 1-st order and return")
		/* revert is not possible, because the funds are already in other wallet
		var err = order.Revert()
		if err != nil {
			prey.logger.Error().Err(err).Msg("****************BALANCE ERROR - REVERT AFTER SWAP - SHOULD NOT HAPPEN")
			prey.logger.Error().Err(err).Msg("prey 1-st order revert failed")
		}
		*/
		pr.printEndLog(prey.h.store)
		return
	} else {
		prey.logger.Info().Msgf("REAL swap return %s", sres.Amount)
	}
	time.Sleep(400 * time.Millisecond) // wait for swap result arrival into accont
	exAcc, poolAcc := prey.h.ob.GetExchange().GetAccount(), prey.h.pool.GetExchange().GetAccount()
	exAcc.Refresh(); if poolAcc != exAcc { poolAcc.Refresh() }
	pr.printEndLog(prey.h.store)
	resultInRune := pr.getResultInRune()
	prey.logger.Info().Msgf("RESULT IN RUNE: %f", resultInRune)
	waitForBalance2AndRebalance := false
	err := prey.balanceAccounts(&ores, &sres, waitForBalance2AndRebalance, alreadyBalanced)
	if err != nil {
		prey.logger.Error().Err(err).Msg("****************BALANCE ERROR - BALANCE 3. - SHOULD NOT HAPPEN")
	}
	if resultInRune < 0 {
		prey.logger.Info().Msg("RESULT NEGATIVE")
	}
	if resultInRune < -5.0 {
		prey.logger.Info().Msgf("RESULT TOO NEGATIVE (%f) - PANIC", resultInRune)
		panic("RESULT TOO NEGATIVE - PANIC")
	}
	if pr.getGlobalResultInRune() < -10.0 {
		prey.logger.Info().Msgf("GLOBAL RESULT TOO NEGATIVE (%f) - PANIC", pr.getGlobalResultInRune())
		panic("GLOBAL RESULT TOO NEGATIVE - PANIC")
	}
	if waitForBalance2AndRebalance {
		exAcc.Refresh(); poolAcc.Refresh()
		err = prey.h.balancer.balanceMarket(exAcc, poolAcc, prey.h.ob.GetMarket())
		if err != nil {
			prey.logger.Error().Err(err).Msg("****************BALANCE ERROR - BALANCE 4. - SHOULD NOT HAPPEN")
		}
	}
}
func getSideAmount(orderSide common.OrderSide, baseAmount, quoteAmount common.Uint/*, flip bool*/) (side common.SwapTo, amount common.Uint) {
	/*
	if flip && orderSide == common.OrderSideBUY {
		side, amount = common.SwapToAsset, baseAmount
	} else if flip && orderSide == common.OrderSideSELL {
		side, amount = common.SwapToQuote, quoteAmount
	} else if !flip && orderSide == common.OrderSideBUY {
		side, amount = common.SwapToQuote, baseAmount
	} else if !flip && orderSide == common.OrderSideSELL {
		side, amount = common.SwapToAsset, quoteAmount
	}
	*/
	if orderSide == common.OrderSideBUY {
		side, amount = common.SwapToQuote, baseAmount
	} else if orderSide == common.OrderSideSELL {
		side, amount = common.SwapToAsset, quoteAmount
	}
	return side, amount
}
func (prey *Prey) balanceFirstAccounts(ores *common.Result) error {
	return prey.balanceAccounts(ores, &common.Result{}, true, 2)
}
func (prey *Prey) balanceAccounts(ores, sres *common.Result, wait bool, alreadyBalanced uint8) (err error) {
	exAcc := prey.h.ob.GetExchange().GetAccount()
	poolAcc := prey.h.pool.GetExchange().GetAccount()
	if exAcc != poolAcc {
		srcBaseAsset := prey.h.ob.GetMarket().BaseAsset
		srcQuoteAsset := prey.h.ob.GetMarket().QuoteAsset
		dstBaseAsset := prey.h.pool.GetMarket().BaseAsset
		dstQuoteAsset := prey.h.pool.GetMarket().QuoteAsset
		var err1 error
		var src, dst common.Asset
		var am1, am2 common.Uint
		var sent1, sent2 *common.Uint
		if prey.side == common.OrderSideSELL { // ex:ETH/BNB <--> pool:ETH/BNB  // flip: ex: RUNE/USDT <--> pool:USDT/RUNE
			src = srcQuoteAsset
			sent1, sent2 = &ores.QuoteAmount, &sres.Amount
			am1, am2 = ores.QuoteAmount, sres.Amount
			dst = dstBaseAsset//; if flip { dst = dstQuoteAsset }
		} else {
			src = srcBaseAsset
			sent1, sent2 = &ores.Amount, &sres.QuoteAmount
			am1, am2 = ores.Amount, sres.Amount
			dst = dstQuoteAsset//; if flip { dst = dstBaseAsset }
		}
		var sent common.Uint
		if alreadyBalanced & 1 == 0 { 
			prey.logger.Info().Msgf("Balancing (1) ex -> pool %s %s", am1, src.Ticker)
			_, sent, err = exAcc.Send(am1, src, poolAcc, wait) 
			if err != nil {
				err = errors.Wrapf(err, "Balancing (1) ERROR ex -> pool %s %s", am1, src.Ticker)
				prey.logger.Error().Err(err).Msg("error")	
				panic(0)
			} else {
				prey.logger.Info().Msgf("Balancing (1) SUCCESS ex -> pool %s (sent %s) %s", am1, sent, src.Ticker)
			}
			*sent1 = sent
		}
		if alreadyBalanced & 2 == 0 { 
			prey.logger.Info().Msgf("Balancing (2) ex <- pool %s %s", am2, dst.Ticker)
			_, sent, err1 = poolAcc.Send(am2, dst, exAcc, wait) 
			if err1 != nil {
				err1 = errors.Wrapf(err1, "Balancing (2) ERROR ex -> pool %s %s", am2, dst.Ticker)
				prey.logger.Error().Err(err1).Msg("error")
				panic(0)
			} else {
				prey.logger.Info().Msgf("Balancing (2) SUCCESS ex -> pool %s (sent %s) %s", am2, sent, dst.Ticker)
			}
			*sent2 = sent
		}
		if err != nil && err1 != nil {
			err = errors.Wrapf(err, "%s", err1)
		} else if err == nil && err1 != nil {
			err = err1
		}
		if err != nil {
			err = errors.Wrap(err, "failed to send rebalance funds after arb trade")
			prey.logger.Error().Err(err).Msg("error")
			panic(0)
		}
	}
	return err
}
func (prey *Prey) enoughFunds(side common.SwapTo, amount common.Uint) (enough bool) {
	exAcc := prey.h.ob.GetExchange().GetAccount()
	poolAcc := prey.h.pool.GetExchange().GetAccount()
	//_ = exAcc 
	if exAcc == poolAcc { return true } // same bdex wallet
	var a common.Asset
	if side == common.SwapToAsset { 
		a = prey.h.pool.GetMarket().QuoteAsset 
	} else {
		a = prey.h.pool.GetMarket().BaseAsset
	}
	enough = poolAcc.GetBalance(a).GTE(amount)
	prey.logger.Info().Msgf(common.IfStr(enough, "We have enough %s on other wallet (needed = %s, have = %s)", "We have NOT enough %s on other wallet (needed = %s, have = %s)"), a.Ticker, amount, poolAcc.GetBalance(a))
	return enough
}