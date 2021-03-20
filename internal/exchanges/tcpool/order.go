package tcpool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/binance-chain/go-sdk/client/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/exchanges/bdex"
)

type orderStatus int8

const (
	ERROR = iota
	WAIT
	OK
)

type Order struct {
	logger zerolog.Logger
	trader	*Swapper
	amount	common.Uint
	side	common.SwapTo
	wallet	string
	hash	string
	millis	int64
	err		error
	result	*common.Result
}

func NewOrder(trader *Swapper, side common.SwapTo, amount common.Uint, wallet string, hash *string, millis int64, err error) *Order {
	o := Order{
		logger: log.With().Str("module", "TC swap order").Str("market", trader.market.String()).Str("dir", side.DirStr(trader.market)).Str("amount", amount.String()).Logger(),
		trader: trader,
		amount: amount,
		wallet:	wallet,
		side:	side,
		millis:	millis,
		err:    err,
	}
	if err == nil && hash == nil {
		o.err = errors.New("TC NewOrder cannot have hash pointer nil and error also nil")
	}
	if hash != nil {
		o.hash = *hash
	}
	return &o
}
func NewErrorOrder(err error) *Order {
	o := Order{
		logger: log.With().Str("module", "TC swap order").Logger(),
		err:    err,
	}
	return &o
}
func (o *Order) GetResult() common.Result {
	if o.err != nil { // if there was a previous error while order sending just return it
		return common.Result{Err: o.err}
	}
	defer o.trader.acc.Refresh()
	if o.result == nil {
		o.result = &common.Result{}
		o.result.QuoteAmount = o.amount
		o.waitForResult()
	}
	return *o.result
}
func (o *Order) Revert() error {
	if o.err != nil || o.result == nil { // if there was a previous error while order sending just return it
		return errors.Wrap(o.err, "cannot call Revert() on previously failed order")
	}
	if o.result.QuoteAmount == common.ZeroUint() { // if there was a previous error while order sending just return it
		return errors.Wrap(o.err, "cannot call Revert() on order with zero QuoteAmount")
	}
	defer o.trader.acc.Refresh()
	return o.PartialRevert(o.result.QuoteAmount)
}
func (o *Order) PartialRevert(amount common.Uint) error {
	if o.err != nil { // if there was a previous error while order sending just return it
		return errors.Wrap(o.err, "cannot call Revert() on previously failed order")
	}
	o2 := o.trader.Swap(o.side.Invert(), amount, common.ZeroUint())
	res := o2.GetResult()
	return res.Err
}
func (o *Order) waitForResult() {
	evch := make(chan struct{}, 1) // account event receiver channel
	quit := make(chan struct{})    // quit channel for SubscribeAccountEvent
	defer close(quit)
	acc := o.trader.acc.(*bdex.Account)
	err := acc.GetDex().SubscribeAccountEvent(o.wallet, quit, func(event *websocket.AccountEvent) { evch <- struct{}{} }, nil, nil)
	if err != nil {
		o.logger.Error().Err(err).Msg("bdex SubscribeAccountEvent failed")
	}
	failed := 0
	var osr orderStatus
	timeout := time.After(24 * time.Hour)
	ticker := time.NewTicker(4 * time.Second)
	for {
		select {
		case <-common.Stop:
			return
		case <-evch:
			time.Sleep(50 * time.Millisecond)
			osr = o.readStatus()
		case <-ticker.C:
			osr = o.readStatus()
		case <-timeout:
			o.result.Err = errors.Wrapf(o.result.Err, "swap timeout, hash = %s", o.hash)
			o.logger.Error().Err(o.result.Err)
			return
		}
		if osr == OK {
			return
		} else if osr == ERROR {
			failed++
			if failed > c.ORDER_RESULT_WAIT_CNT {
				o.result.Err = errors.Wrapf(o.result.Err, "order checkStatus failed %ix for %s", c.ORDER_RESULT_WAIT_CNT, o.trader.market)
				o.logger.Error().Err(o.result.Err)
				return
			}
		}
	}
}
func (o *Order) readStatus() orderStatus {
	// searching for outbond:TxHash
	// https://testnet-dex.binance.org/api/v1/transactions?address=tbnb13egw96d95lldrhwu56dttrpn2fth6cs0axzaad&blockHeight=76990710&startTime=1585724423000
	query := fmt.Sprintf("?address=%v&startTime=%d&side=RECEIVE", o.wallet, o.millis)
	txs, err := getTxs(query)
	for {
		if err != nil {
			if strings.Contains(err.Error(), "read: connection reset by peer") {
				time.Sleep(2 * time.Second)
				txs, err = getTxs(query)
				continue
			}
			o.result.Err = errors.Wrap(err, fmt.Sprintf("swap check failed, tx: %s, query: %s", o.hash, query))
			return WAIT
		}
		break
	}
	searchForOk := "OUTBOUND:" + o.hash
	searchForErr := "REFUND:" + o.hash
	for _, tx := range txs.Tx {
		memo := strings.ToUpper(tx.Memo)
		if strings.Index(memo, searchForOk) != -1 {
			o.result.Amount = common.NewUintFromString(tx.Value)
			o.logger.Info().Str("hash", o.hash).Str("got", o.result.Amount.String()).Msg("swap successfull")
			return OK
		}
		if strings.Index(memo, searchForErr) != -1 {
			o.result.Err = errors.New("swap rejected - refunded")
			o.logger.Info().Str("hash", o.hash).Msg("swap rejected - refunded")
			return ERROR
		}
		//2020-04-14T06:34:03.276Z
		t, err := time.Parse(time.RFC3339, tx.TimeStamp)
		if err != nil {
			o.result.Err = err
			o.logger.Error().Err(err).Str("Tx timestamp", tx.TimeStamp).Msg("Cannot parse time from Tx")
			return WAIT
		}
		t2 := t.UnixNano() / 1000000
		if o.millis < t2 {
			o.millis = t2
		}
	}
	return WAIT // OUTBOUND / REFUND Tx not found yet
}
func getTxs(query string) (*Transactions, error) {
	url := "https://dex.binance.org/api/v1/transactions" + query
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Logger.Error().Err(err).Str("url", url).Msg("getTxs - http.NewRequest error")
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		log.Logger.Error().Err(err).Str("url", url).Msg("getTxs - http.DoHttpRequest error")
		return nil, err
	}
	var res Transactions
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		log.Logger.Error().Err(err).Str("url", url).Msg("getTxs - json.Unmarshal error")
		return nil, err
	}
	return &res, nil
}

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
