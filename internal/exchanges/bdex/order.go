package bdex

import (
	"strings"
	"time"
	"fmt"

	"github.com/binance-chain/go-sdk/client/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
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
	orderId string
	amount  common.Uint
	side    common.OrderSide
	err     error
	result  *common.Result
}

func NewOrder(trader *Trader, side common.OrderSide, amount common.Uint, orderId string, err error) *Order {
	o := Order{
		logger:  log.With().Str("module", "bdex order").Str("market", trader.market.String()).Str("side", side.String()).Str("amount", amount.String()).Logger(),
		trader:  trader,
		side:	 side,
		amount:  amount,
		orderId: orderId,
		err:     err,
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
	if o.result.Err != nil { // if there was a previous error while order sending just return it
		o.logger.Error().Err(o.result.Err).Msgf("order failed")
	} else {
		if o.side == common.OrderSideBUY {
			o.logger.Info().Str("spent", o.result.QuoteAmount.String()).Str("got", o.result.Amount.String()).Str("price", o.result.AvgPrice.String()).Str("partial", fmt.Sprintf("%v",o.result.PartialFill)).Msgf("order successful")
		} else {
			o.logger.Info().Str("spent", o.result.Amount.String()).Str("got", o.result.QuoteAmount.String()).Str("price", o.result.AvgPrice.String()).Str("partial", fmt.Sprintf("%v",o.result.PartialFill)).Msgf("order successful")
		}
	}
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
	side := o.side.Invert()
	var price common.Uint
	if side == common.OrderSideBUY {
		price = o.result.AvgPrice.MulUint64(2)
	} else {
		price = o.result.AvgPrice.QuoUint64(2)
	}
	o2 := o.trader.Trade(side, amount, price)
	res := o2.GetResult()
	return res.Err
}
func (o *Order) waitForResult() {
	o.result = &common.Result{}
	event := make(chan []*websocket.OrderEvent)
	quit := make(chan struct{})
	defer close(quit)
	common.Try(3, 100, func() error {// ignore err
			return o.trader.bdex.dex.SubscribeOrderEvent(o.trader.acc.wallet, quit,
				func(ev []*websocket.OrderEvent) {
					event <- ev
				},
				func(err error) {
					o.logger.Error().Err(err).Msg("SubscribeOrderEvent error event")
				},
				func() {
					o.logger.Debug().Msg("SubscribeOrderEvent closed")
				},
			)
		},
	)
	ticker := time.NewTicker(200 * time.Millisecond)
	failed := 0
	for {
		select {
		case <-common.Quit:
			return
		case evs := <-event:
			for i := 0; i < len(evs); i++ {
				if evs[i].OrderID != o.orderId {
					continue
				}
				status := o.statusFromStr(evs[i].CurrentOrderStatus)
				if status != WAIT {
					o.logger.Debug().Msgf("SubscribeOrderEvent event (to compare quantities and prices when more filled prices): %+v", evs[i])
					o.result.Amount = common.NewUintFromFx8(evs[i].CommulativeFilledQty)
					o.trader.applyTraderFee(&o.result.Amount)
					o.result.AvgPrice = common.NewUintFromFx8(evs[i].LastExecutedPrice)
					o.result.QuoteAmount = o.result.Amount.Mul(o.result.AvgPrice)
					o.result.PartialFill = o.amount != o.result.Amount
					return
				}
				break
			}
		case <-ticker.C:
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
}
func (o *Order) readStatus() orderStatus {
	order, err := o.trader.bdex.dex.GetOrder(o.orderId)
	if err != nil {
		o.result.Err = errors.Wrap(err, "Dex.GetOrder error")
		return ERROR
	}
	status := o.statusFromStr(order.Status)
	if status != WAIT {
		o.result.Amount = common.NewUintFromString(order.CumulateQuantity)
		o.result.AvgPrice = common.NewUintFromString(order.Price)
		o.result.QuoteAmount = o.result.Amount.Mul(o.result.AvgPrice)
		o.result.PartialFill = o.amount != o.result.Amount
	}
	return status
}
func (o *Order) statusFromStr(status string) orderStatus {
	if strings.EqualFold(status, "Ack") {
		return WAIT
	} else if strings.EqualFold(status, "FullyFill") || strings.EqualFold(status, "PartialFill") || strings.EqualFold(status, "IocNoFill") || strings.EqualFold(status, "IocExpire") {
		return OK
	}
	return ERROR
}
