package tcpool

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/c"
)

type Swapper struct {
	logger  zerolog.Logger
	tcpools *tcpools
	market  common.Market
	acc     common.Account
	side    common.SwapTo
	orderId string
}

func NewSwapper(tcpools *tcpools, market common.Market) common.Swapper {
	t := Swapper{
		logger:  log.With().Str("module", "tcpools swapper").Str("market", market.String()).Logger(),
		tcpools: tcpools,
		market:  market,
		acc:     tcpools.acc,
	}
	return &t
}
func (t *Swapper) Swap(side common.SwapTo, amount, limit common.Uint) common.Order {
	t.logger = t.logger.With().Str("dir", side.DirStr(t.market)).Logger()
	t.logger.Info().Str("amount", amount.String()).Msg("Swap started")
	t.side = side
	defer t.acc.Refresh()
	srcAsset, dstAsset := t.market.QuoteAsset, t.market.BaseAsset
	if t.side == common.SwapToQuote {
		srcAsset, dstAsset = dstAsset, srcAsset
	}
	runeAmount := t.tcpools.GetRuneValueOf(amount, srcAsset)
	if runeAmount < c.MIN_SWAP_RUNE {
		return NewErrorOrder(errors.New(fmt.Sprintf("swap failed %s->%s, amount: %s, that is: %s RUNE, which is less than %v RUNE", srcAsset, dstAsset, amount, runeAmount, c.MIN_SWAP_RUNE)))
	}
	var millis24h int64 = 24 * 60 * 60 * 1000
	millis := (time.Now().UnixNano() - millis24h) / 1000000
	dstAcc, err := NewTCAccount(t.tcpools.thor, dstAsset, limit)
	if err != nil {
		return NewErrorOrder(errors.Wrapf(err, "swap failed %s->%s, cannot create destination / memo", srcAsset, dstAsset))
	}
	hash, _, err := t.acc.Send(amount, srcAsset, dstAcc, false)
	if err != nil {
		return NewErrorOrder(errors.Wrapf(err, "swap failed %s->%s, amount: %s", srcAsset, dstAsset, amount))
	}
	deps := t.acc.GetDepositAddresses(common.BNBAsset) // just to get account address - default asset/chain - BNB.BNB
	addr, found := deps[BASECHAIN.String()]
	if !found {
		return NewErrorOrder(errors.New(fmt.Sprintf("account chain inconsistency - this shouldn't happen - contact developers (swap failed %s->%s, amount: %s)", srcAsset, dstAsset, amount)))
	}
	return NewOrder(t, t.side, amount, addr.Address, hash, millis, err)
}

type TCAccount struct {
	logger zerolog.Logger
	thor   *ThorAddr
	asset  common.Asset
	limit  common.Uint
}
func NewTCAccount(thor *ThorAddr, asset common.Asset, limit common.Uint) (*TCAccount, error) {
	if !asset.Chain.Contains(BASECHAIN) {
		return nil, errors.Errorf("wrong asset %s, doesn't support base chain %s", asset, BASECHAIN)
	}
	return &TCAccount{
		logger: log.With().Str("module", "THORChain vault account").Logger(),
		thor:   thor,
		asset:  asset,
		limit:  limit,
	}, nil
}
func (tca *TCAccount) GetName() string {
	return "THORChain vault"
}
func (tca *TCAccount) GetBalances() common.Balances {
	tca.logger.Panic().Msg("GetBalances disabled")
	return nil
}
func (tca *TCAccount) GetBalance(asset common.Asset) common.Uint {
	tca.logger.Panic().Msg("GetBalance disabled")
	return common.ZeroUint()
}
func (tca *TCAccount) Refresh() common.Balances {
	tca.logger.Panic().Msg("Refresh disabled")
	return nil
}
func (tca *TCAccount) GetDepositAddresses(asset common.Asset) common.DepositAddresses {
	if !asset.Chain.Contains(BASECHAIN) {
		return nil
	}
	memo := "SWAP:" + tca.asset.String()
	if tca.limit > 0 {
		memo = memo + "::LIM:" + tca.limit.ToFx8String()
	}
	return common.DepositAddresses{
		BASECHAIN.String(): common.DepositAddress{
			Address: tca.thor.getAddr(),
			Memo:    memo,
		},
	}
}
func (tca *TCAccount) Send(amount common.Uint, asset common.Asset, target common.Account, wait bool) (txId *string, fee common.Uint, err error) {
	tca.logger.Panic().Msg("Send disabled")
	return nil, common.ZeroUint(), nil
}
func (tca *TCAccount) WaitForDeposit(txID string) error {
	tca.logger.Panic().Msg("WaitForDeposit disabled")
	return errors.New("WaitForDeposit disabled")
}