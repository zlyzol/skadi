package bdex

import (
	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/pkg/errors"
	"gitlab.com/zlyzol/skadi/internal/common"
)

// marketInfo - map["baseSybol_quoteSymbol"] of market infos eg. ticks and lots
type marketInfoMap map[string]marketInfo
type marketInfo struct {
	market	common.Market
	tickLot	common.TickLot
}

func (b *bdex) readMarketInfo() error {
	markets, err := b.dex.GetMarkets(types.NewMarketsQuery().WithLimit(1000))
	if err != nil {
		return errors.Wrap(err, "failed to call GetMarkets")
	}
	b.markets = make(marketInfoMap, len(markets))
	for _, pair := range markets {
		baseAsset, err := common.NewChainAsset(BASECHAIN.String(), pair.BaseAssetSymbol)
		if err != nil {
			b.logger.Info().Err(err).Str("asset", pair.BaseAssetSymbol).Msg("cannot process")
			continue
		}
		// QUOTE asset
		quoteAsset, err := common.NewChainAsset(BASECHAIN.String(), pair.QuoteAssetSymbol)
		if err != nil {
			b.logger.Info().Err(err).Str("asset", pair.QuoteAssetSymbol).Msg("cannot process")
			continue
		}
		mi := marketInfo{
			tickLot:	common.TickLot{ Tick: common.NewUintFromFx8(pair.TickSize), Lot: common.NewUintFromFx8(pair.LotSize) },
			market:		common.NewMarket(baseAsset, quoteAsset),

		}
		// BASE asset
		b.logger.Debug().Str("base", pair.BaseAssetSymbol).Str("quote", pair.QuoteAssetSymbol).Msg("bdex market added")
		b.markets[mi.market.String()] = mi
	}
	return nil
}

