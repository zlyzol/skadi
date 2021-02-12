package common

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// TickLot - tick and lot of particular market
type TickLot struct {
	Tick	Uint
	Lot		Uint
}
type TickLots map[string]TickLot

type Market struct {
	BaseAsset	Asset	`json:"base_asset" mapstructure:"base_asset"`
	QuoteAsset	Asset	`json:"quote_asset" mapstructure:"quote_asset"`
}

func NewMarket(asset, qasset Asset) Market {
	market := Market{
		BaseAsset:	asset,
		QuoteAsset:	qasset,
	}
	return market
}
func (m Market) Equal(m2 Market) bool {
	return m.BaseAsset.Equal(m2.BaseAsset) && m.QuoteAsset.Equal(m2.QuoteAsset)
}
func (m Market) Contains(m2 Market) bool {
	return m.BaseAsset.Contains(m2.BaseAsset) && m.QuoteAsset.Contains(m2.QuoteAsset)
}
func (m Market) GetAssets() (base Asset, quote Asset) {
	return m.BaseAsset, m.QuoteAsset
}
func (m Market) Swap() Market {
	return NewMarket(m.QuoteAsset, m.BaseAsset)
}
func (m Market) String() string {
	return m.BaseAsset.Ticker.String() + "_" + m.QuoteAsset.Ticker.String()
}
func (m Market) FullString() string {
	return m.GetSymbol("_")
}
func (m Market) GetSymbol(separator string) string {
	return fmt.Sprintf("%s%s%s", m.BaseAsset.Symbol.String(), separator, m.QuoteAsset.Symbol.String())
}

func (m Market) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *Market) UnmarshalJSON(data []byte) error {
	var err error
	var marketStr string
	if err := json.Unmarshal(data, &marketStr); err != nil {
		return err
	}
	parts := strings.Split(marketStr, "_")
	if len(parts) != 2 {
		return errors.New(fmt.Sprintf("failed to unmarshal Market, missing underscore: %s)", marketStr))
	}
	a, err := NewAsset(parts[0])
	if err != nil {
		return err
	}
	qa, err := NewAsset(parts[1])
	if err != nil {
		return err
	}
	*m = NewMarket(a, qa)
	return nil
}
