package tcpool

import (
	"strings"

	"gitlab.com/zlyzol/skadi/internal/common"
)

func (tc *tcpools) readMarketInfo() error {
	rawPools, err := tc.thor.getPools()
	if err != nil {
		return err
	}
	tc.markets = make([]common.Market, 0, len(rawPools))
	tc.tickerAssets = make(map[common.Ticker]common.Asset, len(rawPools) + 1)
	for _, raw := range rawPools {
		if !strings.EqualFold(raw.Status, "Enabled") { 
			tc.logger.Info().Str("status", raw.Status).Str("pool", raw.Asset).Str("Status", raw.Status).Msg("pool skipped")
			continue
		}
		asset, err := common.NewChainAsset(BASECHAIN.String(), raw.Asset)
		if err != nil {
			tc.logger.Err(err).Str("asset", raw.Asset).Msg("cannot uset this asset")
			continue
		}
		tc.markets = append(tc.markets, common.NewMarket(asset, common.RuneAsset()))
		tc.logger.Debug().Str("asset", asset.Ticker.String()).Msg("tcpool market added")

		if _, found := tc.tickerAssets[asset.Ticker]; !found {
			tc.tickerAssets[asset.Ticker] = asset
		}
	}
	if _, found := tc.tickerAssets[common.RuneAsset().Ticker]; !found {
		tc.tickerAssets[common.RuneAsset().Ticker] = common.RuneAsset()
	}
	return nil
}
