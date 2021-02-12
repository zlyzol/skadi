package bot

/*
package dexarb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	//	"github.com/Workiva/go-datastructures/threadsafe/err"
	"github.com/binance-chain/go-sdk/client/transaction"
	"github.com/binance-chain/go-sdk/client/websocket"
	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/binance-chain/go-sdk/types/tx"
	"gitlab.com/zlyzol/bepbalancer/binance"

	"gitlab.com/zlyzol/bepbalancer/common"
	"gitlab.com/zlyzol/bepbalancer/poolscn"
)

var BUY = common.OrderSideBUY
var SELL = common.OrderSideSELL
func ex(x int8) string {if x == 0 {return "DEX"} else {return "BIN"}}
func os(op int8) string {if op == BUY{return "BUY"} else {return"SELL"}}

type Transaction struct {
	BlockHeight   int64  `json:"blockHeight,omitempty"` // block height
	Code          int64  `json:"code,omitempty"`        // transaction result code	0
	ConfirmBlocks int64  `json:"confirmBlocks,omitempty"`
	Data          string `json:"data,omitempty"`
	FromAddr      string `json:"fromAddr,omitempty"`  // from address
	OrderId       string `json:"orderId,omitempty"`   // order ID
	TimeStamp     string `json:"timeStamp,omitempty"` // time of transaction
	ToAddr        string `json:"toAddr,omitempty"`    // to address
	TxAge         int64  `json:"txAge,omitempty"`
	TxAsset       string `json:"txAsset,omitempty"`
	TxFee         string `json:"txFee,omitempty"`
	TxHash        string `json:"txHash,omitempty"` // hash of transaction
	TxType        string `json:"txType,omitempty"` // type of transaction
	Value         string `json:"value,omitempty"`  // value of transaction
	Source        int64  `json:"source,omitempty"`
	Sequence      int64  `json:"sequence,omitempty"`
	SwapId        string `json:"swapId,omitempty"` // Optional. Available when the transaction type is one of HTL_TRANSFER, CLAIM_HTL, REFUND_HTL, DEPOSIT_HTL
	ProposalId    string `json:"proposalId,omitempty"`
	Memo          string `json:"memo,omitempty"`
}

type Transactions struct {
	Total int64         `json:"total"` // "total": 2
	Tx    []Transaction `json:"tx"`
}

// TokenBalance structure
type TokenBalance struct {
	Symbol string        `json:"symbol"`
	Free   common.Fixed8 `json:"free"`
	Locked common.Fixed8 `json:"locked"`
	Frozen common.Fixed8 `json:"frozen"`
}

// BalanceAccount definition
type BalanceAccount struct {
	Number    int64          `json:"account_number"`
	Address   string         `json:"address"`
	Balances  []TokenBalance `json:"balances"`
	PublicKey []uint8        `json:"public_key"`
	Sequence  int64          `json:"sequence"`
	Flags     uint64         `json:"flags"`
}

// DexArb - Binance DEX Arbitrage structure
type DexArb struct {
	ctx *common.Context
	ex  int8
	Acc common.AccountService
	// Arbitrage properties
	Asset           string
	ticker			string // short asset - ticker
	TradeType       int8    // OrderSideBUY, OrderSideSELL
	LimitPrice      float64 // in BNB
	TradeSizeBase   float64 // in BNB - total BNB we should balance/arb
	AvgPrice		float64 // in BNB - supposed average price of the Asset
	PoolPrice       float64 // in BNB - pool price of Asset
	BasePriceInRune float64 // BNB price in RUNE calculated from pools

	TradeSizeAsset float64 // total Aset we should balance/arb
	TradeSizeRune  float64 // -""- in RUNE

	// account balance
	base        float64
	rune        float64
	asbal       float64 // asset balance
	orderid     string
	swapAm      float64 // result of swap (amount)
	orderAm     float64 // result of order (amount)
	orderAmBase float64 // result of order (amount) in base (BNB or BUSD)

	BaseAsset 	string
	reversed 	bool
	pool		common.PoolData
	basePool		common.PoolData

	binOrderSymbol	string

	ps *poolscn.PoolScanner // TODO - only for debug (SWAP check)
}

// NewArbTrade - initializes Arbitrage Trade
func NewDexArb(ps *poolscn.PoolScanner, ctx *common.Context, ex int8, asset string, tradeSize float64, limitPrice float64, avgPrice float64, poolPrice float64, tradeType int8, basePriceInRune float64, pool common.PoolData, basePool common.PoolData, reversed bool) (*DexArb, error) {
	if tradeSize*basePriceInRune > common.TradeLimitInRune {
		k := common.TradeLimitInRune / (tradeSize * basePriceInRune)
		tradeSize = k * tradeSize
	}
	a := DexArb{
		ps:	ps,
		ctx:             ctx,
		ex:              ex,
		Asset:           asset,
		ticker:			 common.Ticker(asset),
		TradeType:       tradeType,
		LimitPrice:      limitPrice,  // in BNB
		TradeSizeBase:   tradeSize,   // in BNB
		AvgPrice:     	 avgPrice, // in BNB
		PoolPrice:       poolPrice,   // in BNB
		BasePriceInRune: basePriceInRune,
		TradeSizeAsset:  tradeSize / poolPrice,
		TradeSizeRune:   common.Fl64ToFx8Fl64(tradeSize * basePriceInRune),
		BaseAsset:       "BNB.BNB",
		reversed:		 reversed,
		pool:			 pool,
		basePool:		 basePool,
	}
	if a.ex == 0 {
		a.Acc = NewDexAccount(ctx)
	} else {
		a.Acc = NewBinAccount(ctx)
	}
	if asset == "BNB.BNB" || asset == "BNB.BUSD" {
		a.TradeSizeAsset = a.TradeSizeBase
		a.BaseAsset = asset
		log.Printf("tradeSize (%v) %v, ...Asset %v, ...Rune %v", a.ticker, a.TradeSizeBase, a.TradeSizeAsset, a.TradeSizeRune)
	}
	return &a, nil
}

func (a *DexArb) Arbitrage() int {
	a.getBalance(BalType.All)
	estY := a.estimatedYield()
	log.Printf("ESTIMATED YIELD: %0.4f RUNE", estY)
	if estY < 3 { // yield must be at least 3 RUNE
		log.Printf("yield too low, return")
		return 0
	}
	if a.reversed {
		log.Printf("REVERSED - not properly implemented yet")
		return 0
	}
	log.Printf("   GOGO ARBITRAGE %v", a.Asset)
	defer a.Acc.Refresh()
	if a.Asset == "BNB.BNB" || a.Asset == "BNB.BUSD" {
		if a.ex == 1 {
			log.Printf("BINANCE BNB/BUSD arbitrage")
		}
		return a.arbBase()
	} else {
		if a.ex == 1 {
			log.Printf("BINANCE normal arbitrage")
		}
		return a.arbNorm(estY)
	}
}
func (a *DexArb) estimatedYield() float64 {
	return common.EstimatedYieldLimited(a.Asset, a.TradeSizeAsset, a.AvgPrice, a.TradeType, a.pool, a.basePool, a.Acc, a.BaseAsset)
}

func (a *DexArb) debug_checkPool(d_swapTo int8, d_amount float64) float64 {
	/// DEBUG - TODO - remove
	d_from := "RUNE"
	d_to := common.Ticker(a.Asset)
	if d_swapTo == common.SwapTo.Rune {
		d_from = common.Ticker(a.Asset)
		d_to = "RUNE"
	}
	a.ps.GetPools(a.Asset)
	d_pool := a.ps.Pools[a.Asset]
	d_est := d_pool.SwapReturn(d_amount, d_swapTo)
	log.Printf("SWAP DEBUG EST WAITNG %v %v -> return: %v %v", d_amount, d_from, d_est, d_to)
	/// DEBUG END

	//log.Panic()

	return d_est
}

func (a *DexArb) arbBase() int {
	a.orderid = ""
	if a.TradeType == common.OrderSideSELL { // we want to buy BNB/BUSD on DEX for RUNE (eg. sell RUNE for BNB/BUSD) and sell/swap it to BNB/BUSD pool for RUNE
		log.Printf("Arbitrage path: buy %s on %s for RUNE (eg. sell RUNE for %s) and swap it to %s pool for RUNE", a.ticker, ex(a.ex), a.ticker, a.ticker)
		toSell := math.Min(a.rune, a.TradeSizeRune)
		if toSell < a.TradeSizeRune/2 {
			log.Printf("Info: wallet too small, skipping arbitrage (a.rune, a.TSR = %v, %v)", a.rune, a.TradeSizeRune)
			return 0
		}
		log.Printf("Debug: toSell = %v (a.rune, a.TSR = %v, %v)", toSell, a.rune, a.TradeSizeRune)
		hadAsset := a.asbal

		a.debug_checkPool(common.SwapTo.Rune, toSell * a.LimitPrice)

		if !a.order(SELL, toSell, "BNB.RUNE", a.Asset, a.LimitPrice) {
			return 0
		}
		//toSwap := a.orderAm / a.BasePriceInRune
		toSwap := a.orderAmBase
		sentBack := hadAsset < toSwap
		if sentBack {
			log.Printf("Sendig 1. back from exchange to on chain %v %v", toSwap, a.ticker)
			toSwap = a.Acc.Receive(BalType.Onchain, a.Asset, toSwap)
		}
		if !a.swap(a.Asset, "BNB.RUNE", toSwap, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
		log.Printf("Sendig 2. back to exchange %v RUNE", a.swapAm)
		a.Acc.Receive(BalType.Exchange, "BNB.RUNE", a.swapAm)
		if !sentBack {
			log.Printf("Sendig 3. from exchange to on chain %v %v", toSwap , a.ticker)
			a.Acc.Receive(BalType.Onchain, a.Asset, toSwap)
		}
	} else if a.TradeType == BUY { //// swap RUNE for BNB, sell BNB on DEX for RUNE = buy RUNE for BNB
		log.Printf("Arbitrage path: swap RUNE to %s pool and sell %s on %s for RUNE (eg. buy RUNE for %s)", a.ticker, a.ticker, ex(a.ex), a.ticker)
		toBuy := math.Min(a.TradeSizeRune, a.base * a.BasePriceInRune);
		if toBuy < a.TradeSizeRune/2 {
			log.Printf("Info: wallet too small, skipping arbitrage (a.rune %v, a.base %v, a.TSR %v)", a.rune, a.base, a.TradeSizeRune)
			return 0
		}
		log.Printf("Debug: toBuy=%v, a.TradeSizeRune=%v, a.base=%v, a.BasePriceInRune=%v", toBuy, a.TradeSizeRune, a.base, a.BasePriceInRune)
		hadRune := a.rune

		a.debug_checkPool(common.SwapTo.Asset, toBuy * a.LimitPrice)

		if !a.order(BUY, toBuy, "BNB.RUNE", a.Asset, a.LimitPrice) {
			return 0
		}
		toSwap := a.orderAm
		sentBack := hadRune < toSwap
		if sentBack {
			log.Printf("We have not enough RUNE on chain so Receive them all now: %v RUNE", toSwap)
			toSwap = a.Acc.Receive(BalType.Onchain, "BNB.RUNE", toSwap)
		} else {
			log.Printf("We have enough RUNE on chain so not Receive them all now: %v RUNE", toSwap)
		}
		if !a.swap("BNB.RUNE", a.Asset, toSwap, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
		log.Printf("Sendig 4. back to exchange %v %v", a.swapAm, a.ticker)
		a.Acc.Receive(BalType.Exchange, a.Asset, a.swapAm)
		if !sentBack {
			log.Printf("Sendig 5. back to on chain %v RUNE", toSwap)
			a.Acc.Receive(BalType.Onchain, "BNB.RUNE", toSwap)
		}
	}
	return 1
}

// arbNorm executes the pool balancing / arbitrage trade
func (a *DexArb) arbNorm(estY float64) int {
	if _, found := a.ctx.TickLot[a.ex][a.Asset + "_" + "BNB.BNB"]; !found {
		log.Printf("%v - this asset has not BNB trading pair on %v - cannot use current arbNorm() func", a.ticker, ex(a.ex))
		return 0
	}
	a.orderid = ""
	if a.TradeType == common.OrderSideSELL { // swap RUNE (and BNB if needed) for asset and sell asset on DEX
		log.Printf("PATH 1: swap %v RUNE for %v and sell it on %v", a.TradeSizeRune, a.ticker, ex(a.ex))

		toSwap := math.Min(a.rune, a.TradeSizeRune)
		if toSwap < a.TradeSizeRune/2 {
			log.Printf("Info: wallet too small, skipping arbitrage (a.rune %v, a.TSR %v)", a.rune, a.TradeSizeRune)
			return 0
		}
		// NOT SUPPORTED

		if a.ex == 1 && estY < 30 {
			log.Printf("This type of trade we don't support now for less then 30 RUNE yield (yield: %v) on CEX - swap rune for asset (long duration) & then sell", estY)
			return 0
		} else if a.ex == 0 && estY < 10 {
			log.Printf("This type of trade we don't support now for less then 10 RUNE yield (yield: %v) on DEX - swap rune for asset (long duration) & then sell", estY)
			return 0
		}
		if !a.swap("BNB.RUNE", a.Asset, toSwap, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
		toSell := math.Min(a.swapAm, a.TradeSizeAsset)
		log.Printf("Sendig 10. from exchange to on chain %v %s", toSell, a.ticker)
		a.Acc.Receive(BalType.Exchange, a.Asset, toSell)
		if !a.order(SELL, toSell, a.Asset, "BNB.BNB", a.LimitPrice) {
			log.Printf("Order not executed, swapping back %v %v to RUNE", a.swapAm, a.ticker)
			if !a.swap(a.Asset, "BNB.RUNE", a.swapAm, 0) { // swap back if not sold
				log.Printf("SWAP ERROR")
				return 0
			}
			return 2
		}
		toSwap = a.orderAmBase // amount in BNB
		log.Printf("Sendig 11. back to on chain %v BNB", toSwap)
		a.Acc.Receive(BalType.Onchain, "BNB.BNB", toSwap)
		if !a.swap("BNB.BNB", "BNB.RUNE", toSwap, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
	} else if a.TradeType == common.OrderSideBUY { //// 2. (swap RUNE for BNB if needed) than buy asset on DEX and swap it back for RUNE?
		log.Printf("PATH 2: buy %v on %v and swap it back for RUNE", a.ticker, ex(a.ex))
		//TradeSizeAsset:  tradeSize / poolPrice,
		toBuyBase := math.Min(a.base, a.TradeSizeBase)
		if toBuyBase < a.TradeSizeBase/2 {
			log.Printf("Info: wallet too small, skipping arbitrage (a.base %v, a.TSR %v)", toBuyBase, a.TradeSizeBase)
			return 0
		}
		toBuy := toBuyBase / a.PoolPrice
		log.Printf("PATH 2: buy %v %s on %v", toBuy, a.ticker, ex(a.ex))

		a.debug_checkPool(common.SwapTo.Rune, toBuy * a.LimitPrice)

		if !a.order(BUY, toBuy, a.Asset, "BNB.BNB", a.LimitPrice) {
			return 0
		}
		toSwap := a.orderAm
		log.Printf("Sendig 12. from exchange to on chain %v %v", a.orderAm, a.ticker)
		a.Acc.Receive(BalType.Onchain, a.Asset, a.orderAm)
		if !a.swap(a.Asset, "BNB.RUNE", toSwap, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
		if !a.swap("BNB.RUNE", "BNB.BNB", a.swapAm, 0) {
			log.Printf("SWAP ERROR")
			return 0
		}
		a.Acc.Receive(BalType.Exchange, "BNB.BNB", a.swapAm)
	}
	return 1
}

func (a *DexArb) SwapWrapper(what, forWhat string, amount, limit float64) bool {
	return a.swap(what, forWhat, amount, limit)
}

func (a *DexArb) order(op int8, amount float64, asset, qasset string, price float64) bool {
	amountInRune := a.amountInRune
	if amountInRune(asset, amount) < 10 {
		log.Printf("%v - ERROR - too low amount [%v]", a.ticker, amountInRune(asset, amount))
		return false
	}
	err := a.CreateOrder(asset, qasset, op, price, amount, true)
	if err != nil {
		log.Printf("ERROR: Cannot make the trade on %v - a.ctx.Dex.CreateOrder error: [%v]", ex(a.ex), err)
		log.Printf("CreateOrder(asset=%v, qasset=%v, op=%v, price=%v, amount=%v)", asset, qasset, op, price, amount)
		return false
	}
	if !a.getOrderOutcome() {
		return false
	}
	if a.orderAm == 0 {
		return false
	}
	a.Acc.Refresh()
	return true
}
func (a *DexArb) amountInRune(asset string, amount float64) float64 {
	amountInRune := amount * a.PoolPrice * a.BasePriceInRune
	if asset != a.Asset {
		if asset == "BNB.RUNE" {
			amountInRune = amount
		} else {
			amountInRune = amount * a.BasePriceInRune
		}
	}
	return amountInRune
}

func (a *DexArb) swap(asset, forAsset string, amount, limit float64) bool {
	if amount = a.Acc.PrepareFast(BalType.Onchain, asset, amount); amount == 0 {return false}
	if a.amountInRune(asset, amount) < 10 {
		log.Printf("Too small amount to swap (%v) - not swaping", a.amountInRune(asset, amount))
		return false
	}

	long := a.ctx.LongAsset[asset]
	_, symbol, _ := common.DecomposeAsset(long)
	longFor := a.ctx.LongAsset[forAsset]
	a.swapAm = 0
	m := "SWAP:" + longFor
	if limit > 0 {
		m = m + "::LIM:" + common.Fl64ToFx8(limit).String()
	}

		var d_est float64
		if asset == "BNB.RUNE" {
			d_est = a.debug_checkPool(common.SwapTo.Asset, amount)
		} else {
			d_est = a.debug_checkPool(common.SwapTo.Rune, amount)
		}


	log.Printf("SWAP %v %v for %v, limit: %v (memo: %v) ", amount, common.Ticker(long), common.Ticker(longFor), limit, m)
	var millis24h int64 = 24 * 60 * 60 * 1000
	millis := (time.Now().UnixNano() - millis24h) / 1000000
	msgs := []msg.Transfer{{
		ToAddr: a.ctx.ThorAddr.GetAcc(),
		Coins:  types.Coins{types.Coin{Denom: symbol, Amount: common.Fl64ToFxI64(amount)}},
	}}
	txres, err := a.ctx.Dex.SendToken(msgs, true, transaction.WithMemo(m))
	if err != nil {
		if !strings.Contains(err.Error(), "Invalid sequence.") {
			log.Printf("ERROR: swap - a.ctx.Dex.SendToken call failed. Error: [%v]", err)
			return false
		}
		a.Acc.Refresh()
		for i := 0; i < 10 && strings.Contains(err.Error(), "Invalid sequence."); i++ {
			seq, num := a.Acc.IncrementSequence()
			txres, err = a.ctx.Dex.SendToken(msgs, true, transaction.WithMemo(m), transaction.WithAcNumAndSequence(num, seq+1))
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Printf("ERROR: swap - Cannot swap %s for %s for trade (memo:[%s]). a.ctx.Dex.SendToken error: [%v]", long, longFor, m, err)
			return false
		}
	}
	err = a.getSwapRes(strings.ToUpper(txres.Hash), millis)
	a.Acc.Refresh()
	if err != nil {
		log.Printf("ERROR: swap - Cannot swap %s for %s for trade (memo:[%s]). getSwapRes error: [%v]", long, longFor, m, err)
		return false
	}

		/// DEBUG - TODO - remove
		log.Printf("SWAP DEBUG EST RESULT %v %v -> return: %v %v", d_est, common.Ticker(longFor), a.swapAm, common.Ticker(longFor))
		d_diff := math.Abs((d_est - a.swapAm) / d_est) * 100
		if d_diff > 1 {
			log.Printf("SWAP DEBUG EST DIF BIG %v %%", d_diff)
		}
		/// DEBUG END


	log.Printf("SWAP OK %.6f %v, ret %.6f %v, limit: %v ", amount, common.Ticker(long), a.swapAm, common.Ticker(longFor), limit)
	return a.swapAm > 0
}
func (a *DexArb) getSwapRes(txHash string, fromMillis int64) error {
	ch := make(chan struct{}, 1)  // account event receiver channel
	chQ := make(chan struct{}, 3) // quit channel for SubscribeAccountEvent
	err := a.ctx.Dex.SubscribeAccountEvent(a.ctx.TradingWallet, chQ, func(event *websocket.AccountEvent) {
		ch <- struct{}{}
	}, nil, nil)
	if err != nil {
		return err
	}
	defer func() {
		close(chQ) // chQ <- struct{}{}; chQ <- struct{}{}; chQ <- struct{}{}
	}()
	timeout := time.After(24 * time.Hour)
	ticker := time.NewTicker(4 * time.Second)
	dbgticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ch:
			//log.Print("Swap result check on account event")
			time.Sleep(100 * time.Millisecond)
			ok, err := a.checkSwapRes(txHash, &fromMillis)
			if err != nil {
				return err
			}
			if ok {
				//log.Print("Swap result confirmed on account event")
				return nil
			}
		case <-ticker.C:
			//log.Print("Swap result check ticker")
			ok, err := a.checkSwapRes(txHash, &fromMillis)
			if err != nil {
				return err
			}
			if ok {
				//log.Print("Swap result confirmed on ticker")
				return nil
			}
		case <-dbgticker.C:
			//log.Print("Swap result check ticker")
			ok, err := a.checkSwapRes(txHash, &fromMillis)
			if err != nil {
				return err
			}
			if ok {
				//log.Print("Swap result confirmed on ticker")
				return nil
			}
			log.Printf("Still no Swap result for %v", txHash)
			log.Printf("https://"+a.ctx.DexURL+"/api/v1/transactions?address=%v&startTime=%d&side=RECEIVE", a.ctx.TradingWallet, fromMillis)
		case <-timeout:
			log.Printf("Swap result check on timeout for %v", txHash)
			log.Printf("https://"+a.ctx.DexURL+"/api/v1/transactions?address=%v&startTime=%d&side=RECEIVE", a.ctx.TradingWallet, fromMillis)
			ok, err := a.checkSwapRes(txHash, &fromMillis)
			if err != nil {
				log.Panicf("getSwapRes() - TIMEOUT ERROR: %v", err)
				//return err
			}
			if ok {
				log.Print("Swap result confirmed on timeout")
				return nil
			}
			log.Panicf("getSwapRes() - TIMEOUT ERROR for %v - it seems NOT to be swapped after %v hour", txHash, 24)
		}
	}
}
func (a *DexArb) checkSwapRes(txHash string, fromMillis *int64) (bool, error) {
	// searching for outbond:TxHash
	// https://testnet-dex.binance.org/api/v1/transactions?address=tbnb13egw96d95lldrhwu56dttrpn2fth6cs0axzaad&blockHeight=76990710&startTime=1585724423000
	query := fmt.Sprintf("?address=%v&startTime=%d&side=RECEIVE", a.ctx.TradingWallet, *fromMillis)
	txs, err := a.GetTransactions(query)
	for {
		if err != nil {
			if strings.Contains(err.Error(), "read: connection reset by peer") {
				time.Sleep(2 * time.Second)
				txs, err = a.GetTransactions(query)
				continue
			}
			log.Printf("SWAP check error: %v", query)
			return false, err
		}
		break
	}
	searchForOk := "OUTBOUND:" + txHash
	searchForErr := "REFUND:" + txHash
	for _, tx := range txs.Tx {
		m := strings.ToUpper(tx.Memo)
		if strings.Index(m, searchForOk) != -1 {
			a.swapAm, _ = strconv.ParseFloat(tx.Value, 64)
			log.Printf("SWAP confirmation - returned %.6f %v, (memo:%v) (id:%v)", a.swapAm, tx.TxAsset, m, tx.TxHash)
			return true, nil
		}
		m = strings.ToUpper(tx.Memo)
		if strings.Index(m, searchForErr) != -1 {
			log.Printf("SWAP rejection detected [%v] (id:%v)", m, tx.TxHash)
			return false, fmt.Errorf("Swap rejected [%v]", m)
		}
		//2020-04-14T06:34:03.276Z
		t, err := time.Parse(time.RFC3339, tx.TimeStamp)
		if err != nil {
			log.Printf("Cannot parse time from Tx: %v", tx.TimeStamp)
			return false, err
		}
		t2 := t.UnixNano() / 1000000
		if *fromMillis < t2 {
			*fromMillis = t2 + 1 // +1 millisecond not to repeatedly read the same Tx
		}
	}
	// OUTBOUND Tx not found yet
	return false, nil
}

func (a *DexArb) GetTransactions(query string) (*Transactions, error) {
	url := "https://" + a.ctx.DexURL + "/api/v1/transactions" + query
	//log.Printf("GetTransactions - calling %v", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("GetTransactions - http.NewRequest error: %v", err)
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		log.Printf("GetTransactions - common.DoHttpRequest error: %v", err)
		return nil, err
	}
	var res Transactions
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		log.Printf("GetTransactions - json.Unmarshal error: %v", err)
		return nil, err
	}
	return &res, nil
}

func (a *DexArb) getOrderOutcome() bool {
	ticker := time.NewTicker(1 * time.Second)
	failed := 0
	for {
		select {
		case <-ticker.C:
			osr := a.getOrderStatusResut("")
			if osr == 1 {
				return true
			} else if osr == -1 {
				failed++
				if failed > 10 {
				log.Printf("ERROR getOrderOutcome - failed 10x")
				return false
				}
				//log.Printf("getOrderOutcome - do nothing, maybe next time")
				//return false
			}
		case <-time.After(10 * time.Second):
			osr := a.getOrderStatusResut("")
			if osr == 1 {
				//log.Printf("WARNING - dexarb.go getOrderOutcome() - TIMEOUT - it seems be bought but after 10 sec")
				return true
			}
			log.Printf("ERROR - dexarb.go getOrderOutcome() - TIMEOUT - it seems NOT to be bought after 10 sec")
			return false
		}
	}
}
func (a *DexArb) getOrderStatusResut(orderStatus string) int {
var debugO *OrderStatus
	a.orderAm, a.orderAmBase = 0, 0
	// orderStatus // "Ack", "Canceled", "Expired", "IocNoFill", "IocExpire", "PartialFill", "FullyFill", "FailedBlocking", "FailedMatching", "Unknown"
	if orderStatus == "" {
		o, err := a.getOrder()
		cnt := 0
		for cnt < 10 && err != nil {
			if !strings.Contains(err.Error(), "msg=Order does not exist.") {
				log.Printf("ERROR: getOrderStatusResut (%v-th / 2) - Cannot get order - GetOrder error: [%v]", cnt, err)
				cnt++
			}
			time.Sleep(300 * time.Millisecond)
			o, err = a.getOrder()
			cnt++
		}
		if err != nil {
			log.Printf("SEVERE ERROR: getOrderStatusResut (LAST (%v)) - Cannot get order - GetOrder error: [%v]", cnt, err)
			return -1 // failed
		}
		debugO = o
		orderStatus = o.status
		a.orderAm = o.amount
		if o.amount == 0 {
			a.orderAmBase = 0
		} else {
			a.orderAmBase = o.quotedAmount
		}
	}
	res := 0
	if orderStatus == "Ack" {
		res = 0 // we do nothing, we wait
	}
	if orderStatus == "FullyFill" || orderStatus == "PartialFill" || orderStatus == "IocNoFill" || orderStatus == "IocExpire" {
		// DEX
		log.Printf("%v ORDER OK Status: [%v], Amount: %v, Amount base: %v (id:%v)", ex(a.ex), orderStatus, a.orderAm, a.orderAmBase, a.orderid)
		res = 1 // success
	} else if orderStatus == "FILLED" || orderStatus == "EXPIRED" || orderStatus == "IocNoFill" || orderStatus == "IocExpire" {
		// CEX
		log.Printf("%v ORDER OK Status: [%v], Amount: %v, Amount base: %v (id:%v)", ex(a.ex), orderStatus, a.orderAm, a.orderAmBase, a.orderid)
		res = 1 // success
	} else {
		log.Printf("%v ORDER FAILED Status: [%v] (id:%v)", ex(a.ex), orderStatus, a.orderid)
		res = -1 // failed
	}
	if res == 1 && a.reversed {
		a.orderAm, a.orderAmBase = a.orderAmBase, a.orderAm
		log.Printf("Reversed result: %v %v = %v %v or vice versa :)", a.orderAm, common.Ticker(a.ticker), a.orderAmBase, common.Ticker(a.BaseAsset))
		log.Printf("Reversed debugO: %+v", debugO)
	}
	return res
}

type OrderStatus struct {
	status	string
	price	float64
	amount	float64
	quotedAmount float64
}

func (a *DexArb) getOrder() (*OrderStatus, error) {
	if a.ex == 0 {
		o, err := a.ctx.Dex.GetOrder(a.orderid)
		if err != nil {
			log.Printf("a.ctx.Dex.GetOrder error: [%v]", err)
			return nil, err
		}
		am, _ := strconv.ParseFloat(o.CumulateQuantity, 64)
		price, _ := strconv.ParseFloat(o.Price, 64)
		log.Printf("               debug dex order:%+v", o)
		return &OrderStatus{status: o.Status, price: price, amount: am, quotedAmount: price * am}, nil
	} else if a.ex == 1 {
		oid64, _ := strconv.ParseInt(a.orderid, 10, 64)
		o, err := a.ctx.Bin.NewGetOrderService().OrderID(oid64).Symbol(a.binOrderSymbol).Do(context.Background())
		if err != nil {
			//log.Printf("a.ctx.Bin.NewGetOrderService error: [%v]", err)
			return nil, err
		}
		amQuoted, _ := strconv.ParseFloat(o.CummulativeQuoteQuantity, 64)
		am, _ := strconv.ParseFloat(o.ExecutedQuantity, 64)
		price := amQuoted / am
		return &OrderStatus{status: string(o.Status), price: price, amount: am, quotedAmount: price * am}, nil
	}
	return nil, fmt.Errorf("Bad exchange type DexArb.ex (%v), alowed 0, 1", a.ex)
}
func (a *DexArb) getBalance(balType int8) {
	a.Acc.Refresh()
	balances := a.Acc.GetBalances(balType)
	a.rune = -1.0
	a.base = -1.0
	a.asbal = -1.0
	for asset, amount := range balances {
		if asset == "BNB.RUNE" || asset == a.BaseAsset || asset == a.Asset {
			//log.Printf("getAccBal - We have %v of [%v]\n", amount, symbol)
			if asset == "BNB.RUNE" {
				a.rune = amount
			}
			if asset == a.BaseAsset {
				if asset == "BNB.BNB" {
					a.base = amount - 0.1
				} else if asset == "BNB.BUSD" {
					a.base = amount - 2
				}
			}
			if asset == a.Asset {
				a.asbal = amount
			}
			if a.rune >= 0 && a.base >= 0 && a.asbal >= 0 {
				break
			}
		}
	}
	if a.rune < 0 {
		a.rune = 0
	}
	if a.base < 0 {
		a.base = 0
	}
	if a.asbal < 0 {
		a.asbal = 0
	}
}
func (a *DexArb) CreateOrder(asset, qasset string, op int8, price, amount float64, sync bool, options ...tx.Option) error {
	if a.reversed {
		asset, qasset = qasset, asset
		amount = amount * a.PoolPrice
		price = 1 / price
		log.Printf("* reversed => means %v %v %v for %s @ %v", os(op), amount, common.Ticker(asset), common.Ticker(qasset), price)
	}
	if a.ex == 1 && op == BUY {
		// split base asset evenly for further orders
		qas := qasset
		qam := price * amount
		qbalEx := a.Acc.GetBalance(BalType.Exchange, qas)
		if qbalEx < qam { // we have less then needed on exchange
			tqas := common.Ticker(qas)
			if tqas != "BNB" && tqas != "BUSD" {
				return fmt.Errorf("BAD asset %v for fund split 50:50. this should not happen", tqas)
			}
			qbalCh := a.Acc.GetBalance(BalType.Onchain, qas)
			if qbalEx < 0.30 * qbalCh { // we have less than 10% on exchange
				xtoReceive := (qbalEx + qbalCh) / 2 - qbalEx
				log.Printf("Info: exchange wallet less then 10%% of total, we split the funds 50:50 -> send %v %v from ex to on", xtoReceive, tqas)
				a.Acc.Receive(BalType.Exchange, qas, xtoReceive)
				return fmt.Errorf("Not enough base asset on exchange wallet - we split them now evenly, so maybe next time, not this time")
			} else if qbalCh < 0.30 * qbalEx {
				xtoReceive := (qbalEx + qbalCh) / 2 - qbalEx
				log.Printf("Info: onchain wallet less then 10%% of total, we split the funds 50:50 -> send %v %v from ex to on", xtoReceive, tqas)
				a.Acc.Receive(BalType.Exchange, qas, xtoReceive)
				return fmt.Errorf("Not enough base asset on exchange wallet - we split them now evenly, so maybe next time, not this time")
			}
		}
	}

	logs := fmt.Sprintf("%v ORDER %v", ex(a.ex), os(op))
	log.Printf("%s %v %s for (lim) %v = %v %s", logs, amount, common.Ticker(asset), price, price*amount, common.Ticker(qasset))
//	log.Printf("BETTER: %s %v %s for (lim) %v = %v %s", logs, amount, common.Ticker(asset), price, price*amount, common.Ticker(qasset))
	var err error
	if a.ex == 0 {
		err = a.dexCreateOrder(asset, qasset, op, price, amount, sync, options...)
	} else {
		err = a.binCreateOrder(asset, qasset, op, price, amount, sync, options...)
	}
	return err
}
func (a *DexArb) binCreateOrder(asset, qasset string, op int8, price, amount float64, sync bool, options ...tx.Option) error {
	if op == SELL {
		amount = a.Acc.PrepareFast(BalType.Exchange, asset, amount)
	} else {
		amount = a.Acc.PrepareFast(BalType.Exchange, qasset, amount * price) / price
	}
	price, amount = a.ctx.AdjustTickLot(a.ex, asset, qasset, price, amount)
	if amount == 0 {return fmt.Errorf("Error: cannot prepare funds (%v, %v)", common.Ticker(qasset), amount * price)}
	_, asset, _ = common.DecomposeAsset(asset)
	_, qasset, _ = common.DecomposeAsset(qasset)
	symbol := fmt.Sprintf("%s%s", asset, qasset)
	//if a.reversed {symbol = fmt.Sprintf("%s%s", qasset, asset)}
	a.binOrderSymbol = symbol
	side := binance.SideTypeBuy
	if op == SELL {
		side = binance.SideTypeSell
	}
	sprice := strconv.FormatFloat(price, 'f', -1, 64)
	squantity := strconv.FormatFloat(amount, 'f', -1, 64)
	order, err := a.ctx.Bin.NewCreateOrderService().Symbol(symbol).
		Side(side).Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeIOC).Quantity(squantity).
		Price(sprice).Do(context.Background())
	if err != nil {
		log.Printf("ERROR NewCreateOrderService symbol=%s amount=%v price=%v", symbol, squantity, sprice)
		return err
	}
	a.orderid = strconv.FormatInt(order.OrderID, 10)
	return nil
}
func (a *DexArb) dexCreateOrder(asset, qasset string, op int8, price, amount float64, sync bool, options ...tx.Option) error {
	if op == SELL {
		amount = a.Acc.PrepareFast(BalType.Onchain, asset, amount)
		if amount == 0 {return fmt.Errorf("Error: cannot prepare funds (%v, %v)", common.Ticker(asset), amount)}
	} else {
		amount = a.Acc.PrepareFast(BalType.Onchain, qasset, amount * price) / price
		if amount == 0 {return fmt.Errorf("Error: cannot prepare funds (%v, %v)", common.Ticker(qasset), amount * price)}
	}
	price, amount = a.ctx.AdjustTickLot(a.ex, asset, qasset, price, amount)
	asset = a.ctx.LongAsset[asset]
	qasset = a.ctx.LongAsset[qasset]
	_, assetSym, _ := common.DecomposeAsset(asset)
	_, qassetSym, _ := common.DecomposeAsset(qasset)
	compoundAsset := fmt.Sprintf("%s_%s", assetSym, qassetSym)
	newOrderMsg := NewCreateOrderMsg(
		a.Acc.GetBech32(),
		"",
		op,
		compoundAsset,
		common.Fl64ToFxI64(price),
		common.Fl64ToFxI64(amount),
	)
	commit, err := a.broadcastMsg(newOrderMsg, sync, options...)
	if err != nil {
		return err
	}
	type commitData struct {
		OrderId string `json:"order_id"`
	}
	var cdata commitData
	if sync {
		err = json.Unmarshal([]byte(commit.Data), &cdata)
		if err != nil {
			return err
		}
	}
	a.orderid = cdata.OrderId
	return nil
}

// NewCreateOrderMsg constructs a new CreateOrderMsg
func NewCreateOrderMsg(sender types.AccAddress, id string, side int8, symbol string, price int64, qty int64) msg.CreateOrderMsg {
	return msg.CreateOrderMsg{
		Sender:    sender,
		ID:        id,
		Symbol:    symbol,
		OrderType: msg.OrderType.LIMIT, // default
		Side:      side,
		Price:     price,
		Quantity:  qty,
		//		TimeInForce: TimeInForce.GTC, // default
		TimeInForce: msg.TimeInForce.IOC, // IOC
	}
}
func (a *DexArb) broadcastMsg(m msg.Msg, sync bool, options ...tx.Option) (*tx.TxCommitResult, error) {
	n, err := a.ctx.Dex.GetNodeInfo()
	if err != nil {
		return nil, err
	}
	// prepare message to sign
	signMsg := &tx.StdSignMsg{
		ChainID:       n.NodeInfo.Network,
		AccountNumber: -1,
		Sequence:      -1,
		Memo:          "",
		Msgs:          []msg.Msg{m},
		Source:        tx.Source,
	}

	for _, op := range options {
		signMsg = op(signMsg)
	}

	if signMsg.Sequence == -1 || signMsg.AccountNumber == -1 {
		acc, err := a.ctx.Dex.GetAccount(a.Acc.GetWallet())
		if err != nil {
			return nil, err
		}
		signMsg.Sequence = acc.Sequence
		signMsg.AccountNumber = acc.Number
	}

	// special logic for createOrder, to save account query
	if orderMsg, ok := m.(msg.CreateOrderMsg); ok {
		orderMsg.ID = msg.GenerateOrderID(signMsg.Sequence+1, a.Acc.GetBech32())
		signMsg.Msgs[0] = orderMsg
	}

	for _, m := range signMsg.Msgs {
		if err := m.ValidateBasic(); err != nil {
			return nil, err
		}
	}

	rawBz, err := a.ctx.Key.Sign(*signMsg)
	if err != nil {
		return nil, err
	}
	// Hex encoded signed transaction, ready to be posted to BncChain API
	hexTx := []byte(hex.EncodeToString(rawBz))
	param := map[string]string{}
	if sync {
		param["sync"] = "true"
	}
	commits, err := a.ctx.Dex.PostTx(hexTx, param)
	if err != nil {
		return nil, err
	}
	if len(commits) < 1 {
		return nil, fmt.Errorf("Len of tx Commit result is less than 1 ")
	}
	return &commits[0], nil
}
*/
