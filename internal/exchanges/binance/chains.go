package binance

import (
	"context"

	"github.com/pkg/errors"
	"gitlab.com/zlyzol/skadi/internal/common"
)
type chainInfoMap map[common.Ticker]chainInfo
type chainInfo struct {
	chain 			common.Chain
	chainDetails	map[string]chainDetail
}
type chainDetail struct {
	withdrawFee		common.Uint
	withdrawMin		common.Uint
	withdrawMul		common.Uint
}
func (b *binance) readCoinChainInfo() error {
	cis, err := b.api.NewCoinInfoService().Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "NewCoinInfoService failed")
	}
	chs := make(chainInfoMap, len(*cis))
	for _, ci := range *cis {
		ticker, err := common.NewTicker(ci.Coin)
		if err != nil {
			b.logger.Panic().Str("asset", ci.Coin).Msg("cannot get ticker - bad coin")
		}
		if ticker == "ETH" || ticker == "POE" {
			a := 0
			_ = a
		}
		nlist := ci.CoinNetworkList()
		chi := chainInfo{}
		chi.chainDetails = make(map[string]chainDetail, len(*nlist))
		someEnabled := false
		for _, cn := range *nlist {
			if !cn.DepositEnable {
				b.logger.Info().Str("asset", ci.Coin).Str("chain", cn.Network).Str("info", cn.DepositDesc).Msg("deposit disabled")
				continue
			}
			if !cn.WithdrawEnable {
				b.logger.Info().Str("asset", ci.Coin).Str("chain", cn.Network).Str("info", cn.WithdrawDesc).Msg("withdraw disabled")
				continue
			}
			someEnabled = true
			chi.chainDetails[cn.Network] = chainDetail{
				withdrawFee: common.NewUintFromString(cn.WithdrawFee),
				withdrawMin: common.NewUintFromString(cn.WithdrawMin),
				withdrawMul: common.NewUintFromString(cn.WithdrawIntegerMultiple),
			}
			if common.NewUintFromString(cn.WithdrawIntegerMultiple).IsZero() {
				a := 0
				_ = a
			}
		}
		if !someEnabled { continue }
		chains := make([]string, len(chi.chainDetails))
		i := 0
		for ch := range chi.chainDetails {
			chains[i] = ch
			i++
		}
		chi.chain, err = common.NewMultiChain(chains)
		if err != nil {
			b.logger.Panic().Err(err).Msgf("cannot assign chains to asset %s, chains: %+v", ci.Coin, chains)
		}
		chs[ticker] = chi
	}
	b.chains = chs
	return nil
}
