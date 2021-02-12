package bdex

import (
	"strings"
	"encoding/hex"
	"encoding/json"

	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"

	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/binance-chain/go-sdk/types/tx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Trader struct {
	logger zerolog.Logger
	bdex   *bdex
	market common.Market
	acc    *Account
}

func NewTrader(bdex *bdex, market common.Market) common.Trader {
	acc, _ := bdex.acc.(*Account)
	t := Trader{
		logger: log.With().Str("module", "bdex trader").Str("market", market.String()).Logger(),
		bdex:   bdex,
		market: market,
		acc:    acc,
	}
	return &t
}
func (t *Trader) Trade(side common.OrderSide, amount, limit common.Uint) common.Order {
	pa := t.adjustTickLot(common.PA{Price: limit, Amount: amount})
	if pa.Amount.Equal(common.ZeroUint()) {
		return &Order{ err: errors.New("order amount is zero after Lot adjusting") }
	}
	if pa.Price.Equal(common.ZeroUint()) {
		return &Order{ err: errors.New("order price is zero after Lot adjusting") }
	}
	orderId, err := t.createOrder(side, pa, true)
	if err == nil {
		t.logger.Info().Msgf("order created: %s %s @ %s", side, pa.Amount, pa.Price)
		t.logger.Debug().Str("orderId", orderId).Msg("error")
	} else {
		t.logger.Error().Err(err).Msgf("order creation failed: %s %s @ %s", side, pa.Amount, pa.Price)
	}
	t.acc.Refresh()
	return NewOrder(t, side, pa.Amount, orderId, err)
}
func (t *Trader) createOrder(side common.OrderSide, pa common.PA, sync bool, options ...tx.Option) (orderId string, err error) {
	var op int8
	if side == common.OrderSideBUY {
		op = msg.OrderSide.BUY
	} else {
		op = msg.OrderSide.SELL
	}
	compoundAsset := t.market.GetSymbol("_")
	newOrderMsg := newCreateOrderMsg(
		t.acc.bech32,
		"",
		op,
		compoundAsset,
		pa.Price.Fx8Int64(),
		pa.Amount.Fx8Int64(),
	)
	commit, err := t.broadcastMsg(newOrderMsg, sync, options...)
	if err != nil {
		for i := 0; i < c.BNB_SEND_TOKEN_INV_SEQ_TRY_CNT && 
		strings.Contains(err.Error(), "Invalid sequence.") || strings.Contains(err.Error(), "Tx already exists in cache"); i++ {
			t.acc.sequenceInc()
			commit, err = t.broadcastMsg(newOrderMsg, sync, options...)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return "", err
	}
	type commitData struct {
		OrderId string `json:"order_id"`
	}
	var cdata commitData
	if sync {
		err = json.Unmarshal([]byte(commit.Data), &cdata)
		if err != nil {
			return "", err
		}
	}
	return cdata.OrderId, nil
}
func (t *Trader) applyTraderFee(amount *common.Uint) {
	coef := common.NewUint(100).Sub(common.NewUintFromFloat(c.BDEX_LIMIT_ORDER_FEE)).Quo(common.NewUint(100))
	*amount = amount.Mul(coef)
}
func newCreateOrderMsg(sender types.AccAddress, id string, side int8, symbol string, price int64, qty int64) msg.Msg {
	return newCreateLimitOrderMsg(sender, id, side, symbol, price, qty)
}

// newCreateLimitOrderMsg constructs a new CreateLimitOrderMsg
func newCreateLimitOrderMsg(sender types.AccAddress, id string, side int8, symbol string, price int64, qty int64) msg.CreateOrderMsg {
	return msg.CreateOrderMsg{
		Sender:    sender,
		ID:        id,
		Symbol:    symbol,
		OrderType: msg.OrderType.LIMIT,
		Side:      side,
		Price:     price,
		Quantity:  qty,
		//		TimeInForce: TimeInForce.GTC, // default
		TimeInForce: msg.TimeInForce.IOC, // IOC
	}
}
/*
// newCreateMarketOrderMsg constructs a new CreateMarketOrderMsg
func newCreateMarketOrderMsg(sender types.AccAddress, id string, side int8, symbol string, qty int64) CreateMarketOrderMsg {
	return msgCreateOrderMsg{
		Sender:    sender,
		ID:        id,
		Symbol:    symbol,
		OrderType: msg.OrderType.MARKET,
		Side:      side,
		Quantity:  qty,
		//		TimeInForce: TimeInForce.GTC, // default
		TimeInForce: msg.TimeInForce.IOC, // IOC
	}
}
*/
func (t *Trader) broadcastMsg(m msg.Msg, sync bool, options ...tx.Option) (*tx.TxCommitResult, error) {
	n, err := t.bdex.dex.GetNodeInfo()
	if err != nil {
		return nil, err
	}
	acc := t.acc
	// prepare message to sign
	signMsg := &tx.StdSignMsg{
		ChainID:       n.NodeInfo.Network,
		AccountNumber: acc.number,
		Sequence:      acc.sequence,
		Memo:          "",
		Msgs:          []msg.Msg{m},
		Source:        tx.Source,
	}
	for _, op := range options {
		signMsg = op(signMsg)
	}
	// special logic for createOrder, to save account query
	if orderMsg, ok := m.(msg.CreateOrderMsg); ok {
		orderMsg.ID = msg.GenerateOrderID(signMsg.Sequence+1, acc.bech32)
		signMsg.Msgs[0] = orderMsg
	}/* else if orderMsg, ok := m.(CreateMarketOrderMsg); ok {
		orderMsg.ID = msg.GenerateOrderID(signMsg.Sequence+1, acc.bech32)
		signMsg.Msgs[0] = orderMsg
	}*/
	for _, m := range signMsg.Msgs {
		if err := m.ValidateBasic(); err != nil {
			return nil, err
		}
	}
	rawBz, err := acc.key.Sign(*signMsg)
	if err != nil {
		return nil, err
	}
	// Hex encoded signed transaction, ready to be posted to BncChain API
	hexTx := []byte(hex.EncodeToString(rawBz))
	param := map[string]string{}
	if sync {
		param["sync"] = "true"
	}
	commits, err := t.bdex.dex.PostTx(hexTx, param)
	if err != nil {
		return nil, err
	}
	if len(commits) < 1 {
		return nil, errors.New("Length of tx Commit result is less than 1 ")
	}
	return &commits[0], nil
}
func (t *Trader) adjustTickLot(pa common.PA) common.PA {
	info, found := t.bdex.markets[t.market.String()]
	if !found {
		t.logger.Panic().Msgf("failed to find marketinfo for [%s]", t.market)
	}
	adjust := func(val common.Uint, tl common.Uint) common.Uint {
		return val.Quo(tl).Floor().Mul(tl)
	}
	return common.PA{Price: adjust(pa.Price, info.tickLot.Tick), Amount: adjust(pa.Amount, info.tickLot.Lot)}
}
