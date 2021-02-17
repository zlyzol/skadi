package bot

import (
	"fmt"
	"time"
	"os"
	"sync/atomic"

	//	"github.com/ethereum/go-ethereum/p2p"
	//	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/store"
	"gitlab.com/zlyzol/skadi/internal/models"
)

type preyResults struct {
	logger			zerolog.Logger
	exAcc, poolAcc	common.Account
	start, end		common.Amounts
	base, quote		common.Asset
	amount 			common.Uint
	expectYield		common.Uint
	/*
	debug struct {
		startEx, endEx, startPool, endPool	common.Amounts
	}
	*/
	resultInRune	float64
	globalResult	float64

}

var global_bigPlus int64 = 0

func newPreyResult(prey *Prey) *preyResults {
	pr := preyResults {
		logger:		log.With().Str("module", "PreyResult").Str("market", prey.h.ob.GetMarket().String()).Logger(),
		exAcc: 		prey.h.ob.GetExchange().GetAccount(),
		poolAcc:	prey.h.pool.GetExchange().GetAccount(),
		base:		prey.h.ob.GetMarket().BaseAsset,
		quote:		prey.h.ob.GetMarket().QuoteAsset,
		amount:		prey.amount,
		expectYield:prey.expectYield,
	}
	/*
	pr.start = common.Amounts{ 
		BaseAmount:		pr.exAcc.GetBalance(pr.base),
		QuoteAmount:	pr.exAcc.GetBalance(pr.quote),
	}
	if pr.exAcc != pr.poolAcc {
		pr.start.BaseAmount += pr.poolAcc.GetBalance(pr.base)
		pr.start.QuoteAmount += pr.poolAcc.GetBalance(pr.quote)
	}
	*/
	pr.printStartLog()
	return &pr
}
func (pr *preyResults) printStartLog() {
	pr.logger.Info().Msg("****************************************")
	pr.logger.Info().Msg(fmt.Sprintf("PREY PROCESSING STARTED %s %s/%s. expYield: %s", pr.exAcc.GetName(), pr.base.Ticker, pr.quote.Ticker, pr.expectYield))
}
func (pr *preyResults) printEndLog(store store.Store) float64 {
	pr.end = common.Amounts{ 
		BaseAmount:		pr.exAcc.GetBalance(pr.base),
		QuoteAmount:	pr.exAcc.GetBalance(pr.quote),
	}
	if pr.exAcc != pr.poolAcc {
		pr.end.BaseAmount += pr.poolAcc.GetBalance(pr.base)
		pr.end.QuoteAmount += pr.poolAcc.GetBalance(pr.quote)
	}
	/*
	pr.debug.endEx = common.Amounts{ 
		BaseAmount:		pr.exAcc.GetBalance(pr.base),
		QuoteAmount:	pr.exAcc.GetBalance(pr.quote),
	}

	pr.debug.endPool = common.Amounts{ 
		BaseAmount:		pr.poolAcc.GetBalance(pr.base),
		QuoteAmount:	pr.poolAcc.GetBalance(pr.quote),
	}
	*/
	startInRune := common.Amounts{
		BaseAmount: common.Oracle.GetRuneValueOf(pr.start.BaseAmount, pr.base), 
		QuoteAmount: common.Oracle.GetRuneValueOf(pr.start.QuoteAmount, pr.quote),
	}
	endInRune := common.Amounts{
		BaseAmount: common.Oracle.GetRuneValueOf(pr.end.BaseAmount, pr.base),
		QuoteAmount: common.Oracle.GetRuneValueOf(pr.end.QuoteAmount, pr.quote),
	}
	pr.resultInRune = endInRune.BaseAmount.Add(endInRune.QuoteAmount).ToFloat() - startInRune.BaseAmount.Add(startInRune.QuoteAmount).ToFloat()
	baseDiff := pr.end.BaseAmount.ToFloat() - pr.start.BaseAmount.ToFloat()
	quoteDiff := pr.end.QuoteAmount.ToFloat() - pr.start.QuoteAmount.ToFloat()
	bigPlus100 := float64(atomic.AddInt64(&global_bigPlus, int64(pr.resultInRune * 100)))
	pr.logger.Info().
		Str("plus", fmt.Sprintf("%.4f", pr.resultInRune)).
		Str(pr.base.Ticker.String(), fmt.Sprintf("%s -> %s (%.4f)", pr.start.BaseAmount, pr.end.BaseAmount, baseDiff)).
		Str(pr.quote.Ticker.String(), fmt.Sprintf("%s -> %s (%.4f)", pr.start.QuoteAmount, pr.end.QuoteAmount, quoteDiff)).
		Msg("prey processing result")
	pr.logger.Info().Msgf("detail prey processing result: %s/%s (%s/%s): %+v", pr.exAcc.GetName(), pr.poolAcc.GetName(), pr.base.Ticker, pr.quote.Ticker, 0/*pr.debug*/)
	pr.logger.Info().Msgf("PREY PROCESSING END %.4f / GLB: %.4f", pr.resultInRune, bigPlus100 / 100)
	pr.logger.Info().Msg("****************************************")

	f, err := os.OpenFile("res.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		now := time.Now(); nows := fmt.Sprintf("%d.%d %d:%02d:%02d", now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
		var s string
		if pr.resultInRune == 0 {
			amInRune := common.Oracle.GetRuneValueOf(pr.amount, pr.base)
			s = fmt.Sprintf("%s (%s/%s): expired (expected trade amount: %s (RUNE), yield: %s)\n", nows, pr.base.Ticker, pr.quote.Ticker, amInRune, pr.expectYield)
		} else {
			s = fmt.Sprintf("%s (%s/%s) (GLB: %.4f): plus %.4f, %s -> %s (%.4f), %s -> %s (%.4f)\n", nows, pr.base.Ticker, pr.quote.Ticker, bigPlus100 / 100, pr.resultInRune, pr.start.BaseAmount, pr.end.BaseAmount, baseDiff, pr.start.QuoteAmount, pr.end.QuoteAmount, quoteDiff)
		}
		f.WriteString(s)
		f.Close()
	}
	pr.globalResult = bigPlus100

	trade := models.Trade{
		Time:	time.Now(),
    	Asset:	pr.base.String(),
    	AmountIn:	startInRune.BaseAmount.Add(startInRune.QuoteAmount).ToFloat(),
		AmountOut:	endInRune.BaseAmount.Add(endInRune.QuoteAmount).ToFloat(),
	    PNL:		pr.resultInRune,
    	Debug:		"cmuk",
	}
	store.InsertTrade(trade)
	return pr.resultInRune
}
