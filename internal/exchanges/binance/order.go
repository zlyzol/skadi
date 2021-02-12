package binance

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/exchanges/binance/api"
)

type orderStatus int8

const (
	ERROR = iota
	WAIT
	OK
)

type Order struct {
	logger  zerolog.Logger
	trader  *Trader
	orderId int64
	amount  common.Uint
	side    common.OrderSide
	err     error
	result  *common.Result
}
func NewOrder(trader *Trader, side common.OrderSide, amount common.Uint, order *api.CreateOrderResponse, err error) *Order {
	o := Order{
		logger:  log.With().Str("module", "binance order").Str("market", trader.market.String()).Str("side", side.String()).Str("amount", amount.String()).Logger(),
		trader:  trader,
		side:	 side,
		amount:  amount,
		err:     err,
	}
	if order != nil {
		o.orderId = order.OrderID
		o.result = &common.Result{
			Err:		nil,
			Amount:		common.NewUintFromString(order.ExecutedQuantity),
			QuoteAmount:common.NewUintFromString(order.CummulativeQuoteQuantity),
			AvgPrice:	common.NewUintFromString(order.Price),
		}
		o.result.PartialFill = !o.amount.Equal(o.result.Amount)
		if o.result.PartialFill { o.logger.Info().Msgf("partial fill because %s != %s", o.amount, o.result.Amount) }
		o.logger.Info().Msgf("successful order amounts (base, quote) %s, %s", o.result.Amount, o.result.QuoteAmount)
		o.trader.applyTraderFee(&o.result.QuoteAmount)
		o.trader.applyTraderFee(&o.result.Amount)
		o.logger.Info().Msgf("successful order amounts after applyTraderFee %s, %s", o.result.Amount, o.result.QuoteAmount)
		if o.result.Amount.IsZero() {
			o.err = errors.New("order expired / not filled")
			o.result = nil
		}
	}
	return &o
}
func (o *Order) GetResult() common.Result {
	if o.err != nil { // if there was a previous error while order sending just return it
		o.logger.Error().Err(o.err).Msgf("order creation failed")
		return common.Result{Err: o.err}
	}
	defer o.trader.acc.Refresh()
	if o.result == nil {
		o.waitForResult()
		if o.result.Amount.IsZero() {
			return common.Result{Err: errors.New("order expired / not filled")}
		}
	}
	if o.err != nil { // if there was a previous error while order sending just return it
		o.logger.Error().Err(o.err).Msgf("order failed")
	}
	o.logger.Info().Msgf("order successful %+v", *o.result)
	return *o.result
}
func (o *Order) Revert() error {
	defer o.trader.acc.Refresh()
	return o.PartialRevert(o.amount)
}
func (o *Order) PartialRevert(amount common.Uint) error {
	if o.result == nil {
		err := errors.New("cannot revert failed order")
		o.logger.Panic().Err(err).Msg("error")
		return err
	}
	o2 := o.trader.Trade(o.side.Invert(), amount, common.ZeroUint())
	res := o2.GetResult()
	return res.Err
}
func (o *Order) waitForResult() {
	o.result = &common.Result{}
	ticker := time.NewTicker(400 * time.Millisecond)
	failed := 0
	for range ticker.C {
		osr := o.readStatus()
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
	compoundAsset := o.trader.market.GetSymbol("")
	order, err := o.trader.bin.api.NewGetOrderService().OrderID(o.orderId).Symbol(compoundAsset).Do(context.Background())
	if err != nil {
		o.result.Err = errors.Wrap(err, "Binance.GetOrder error")
		return ERROR
	}
	status := o.statusFromStr(string(order.Status))
	if status != WAIT {
		o.result.Amount = common.NewUintFromString(order.ExecutedQuantity)
		o.result.QuoteAmount = common.NewUintFromString(order.CummulativeQuoteQuantity)
		o.logger.Info().Msgf("successful order amounts (base, quote) %s, %s", o.result.Amount, o.result.QuoteAmount)
		o.trader.applyTraderFee(&o.result.QuoteAmount)
		o.trader.applyTraderFee(&o.result.Amount)
		o.logger.Info().Msgf("successful order amounts after applyTraderFee %s, %s", o.result.Amount, o.result.QuoteAmount)
		o.result.AvgPrice = o.result.QuoteAmount.Quo(o.result.Amount)
	}
	return status
}
func (o *Order) statusFromStr(status string) orderStatus {
	if strings.EqualFold(status, "Ack") {
		return WAIT
	} else if strings.EqualFold(status, "FILLED") || strings.EqualFold(status, "EXPIRED") || strings.EqualFold(status, "IocNoFill") || strings.EqualFold(status, "IocExpire") {
		return OK
	}
	return ERROR
}
