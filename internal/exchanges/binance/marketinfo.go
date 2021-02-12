package binance

import (
	"context"

	"github.com/pkg/errors"
	"gitlab.com/zlyzol/skadi/internal/common"
)

// marketInfo - map["baseSybol_quoteSymbol"] of market infos eg. ticks and lots
type marketInfoMap map[string]marketInfo
type marketInfo struct {
	market	common.Market
	tickLot	common.TickLot
	minNotional	common.Uint
}
func (b *binance) readMarketInfo() error {
	err := b.readCoinChainInfo()
	if err != nil {
		return errors.Wrap(err, "failed to read readCoinChainInfo")
	}
	ei, err := b.api.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "NewExchangeInfoService failed")
	}
	b.markets = make(marketInfoMap, len(ei.Symbols))
	for _, sym := range ei.Symbols {
		if sym.Status != "TRADING" { continue }
		if !sym.IsSpotTradingAllowed { continue }
		// BASE asset
		baseAsset, err := common.NewAsset(sym.BaseAsset)
		if err != nil {
			b.logger.Info().Str("asset", sym.BaseAsset).Msg("cannot process")
			continue
		}
		chain, found := b.chains[baseAsset.Ticker]
		if !found {
			b.logger.Info().Str("ticker", baseAsset.Ticker.String()).Msg("cannot find chains - probably asset deposit / withdrawal disabled")
			continue
		}
		baseAsset, _ = common.NewMultiChainAsset(chain.chain, baseAsset.Symbol.String()) // we know here won't be error
		// QUOTE asset
		quoteAsset, err := common.NewAsset(sym.QuoteAsset)
		if err != nil {
			b.logger.Info().Str("asset", sym.QuoteAsset).Msg("cannot process")
			continue
		}
		chain, found = b.chains[quoteAsset.Ticker]
		if !found {
			b.logger.Info().Str("asset", quoteAsset.Ticker.String()).Msg("cannot find chains")
			continue
		}
		quoteAsset, _ = common.NewMultiChainAsset(chain.chain, quoteAsset.Symbol.String()) // we know here won't be error
		var tick, lot, min common.Uint
		for _, fil := range sym.Filters {
			if fil["filterType"] == "PRICE_FILTER" {
				tick = common.NewUintFromString(fil["tickSize"].(string))
			} else if fil["filterType"] == "LOT_SIZE" {
				lot = common.NewUintFromString(fil["stepSize"].(string))
			} else if fil["filterType"] == "MIN_NOTIONAL" {
				min = common.NewUintFromString(fil["minNotional"].(string))
			}
		}
		mi := marketInfo{
			market: common.NewMarket(baseAsset, quoteAsset),
			tickLot: common.TickLot{ Tick: tick, Lot: lot },
			minNotional: min,
		}
		b.markets[mi.market.String()] = mi
		b.logger.Debug().Str("base", mi.market.BaseAsset.Ticker.String()).Str("quote", mi.market.QuoteAsset.Ticker.String()).Msg("binance market added")
	}
	return nil
}
