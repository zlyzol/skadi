package binance

import (
	"context"
	"fmt"

	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/binance-chain/go-sdk/types/tx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/exchanges/binance/api"
)

type Trader struct {
	logger  zerolog.Logger
	bin     *binance
	market  common.Market
	acc     *Account
	orderId string
}

func NewTrader(b *binance, market common.Market) common.Trader {
	acc, _ := b.acc.(*Account)
	t := Trader{
		logger: log.With().Str("module", "binance trader").Str("market", market.String()).Logger(),
		bin:    b,
		market: market,
		acc:    acc,
	}
	return &t
}
func (t *Trader) Trade(side common.OrderSide, amount, limit common.Uint) common.Order {
	pa := t.adjustTickLot(common.PA{Price: limit, Amount: amount})
	err := t.checkMinNotional(pa)
	if err != nil { return &Order{ err: err }}
	order, err := t.createOrder(side, pa, true)
	t.acc.Refresh()
	return NewOrder(t, side, pa.Amount, order, err)
}
func (t *Trader) createOrder(side common.OrderSide, pa common.PA, sync bool, options ...tx.Option) (order *api.CreateOrderResponse, err error) {
	var op int8
	if side == common.OrderSideBUY {
		op = msg.OrderSide.BUY
	} else {
		op = msg.OrderSide.SELL
	}
	compoundAsset := t.market.GetSymbol("")
	apiSide := api.SideTypeBuy
	if op == msg.OrderSide.SELL {
		apiSide = api.SideTypeSell
	}
	order, err = t.bin.api.NewCreateOrderService().Symbol(compoundAsset).
		Side(apiSide).Type(api.OrderTypeLimit).
		TimeInForce(api.TimeInForceTypeFOK).Quantity(pa.Amount.String()).
		Price(pa.Price.String()).Do(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "NewCreateOrderService failed symbol=%s amount@price=%s", compoundAsset, pa)
	}
	//t.logger.Info().Msgf("NewCreateOrderService result  can we use it ??? : [%+v]", order)
	return order, nil
}
func (t *Trader) adjustTickLot(pa common.PA) common.PA {
	info, found := t.bin.markets[t.market.String()]
	if !found {
		t.logger.Panic().Msgf("failed to find marketinfo for air [%s]", t.market)
	}
	adjust := func(val common.Uint, tl common.Uint) common.Uint {
		return val.Quo(tl).Floor().Mul(tl)
	}
	return common.PA{Price: adjust(pa.Price, info.tickLot.Tick), Amount: adjust(pa.Amount, info.tickLot.Lot)}
}
func (t *Trader) checkMinNotional(pa common.PA) error {
	info, found := t.bin.markets[t.market.String()]
	if !found {
		t.logger.Panic().Msgf("failed to find marketinfo for air [%s]", t.market)
	}
	if pa.Mul().LT(info.minNotional) {
		return errors.New(fmt.Sprintf("%s order amount*price (%s) is less then MIN_NOTIONAL (%s)", t.market, pa, info.minNotional))
	}
	return nil
}
func (t *Trader) applyTraderFee(amount *common.Uint) {
	return // trader fees are paid from BNB wallet
	//coef := common.OneUint().Sub(common.NewUintFromFloat(c.BINANCE_LIMIT_ORDER_FEE))
	//*amount = amount.Mul(coef)
}
