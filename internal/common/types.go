package common

import (
	"fmt"
	"strconv"

	"github.com/rs/zerolog/log"
)

// Global Quit Channel
var Quit = make(chan struct{})
var Stop chan struct{}

// Balances - account balances
type Balances map[string]Uint
func (balances Balances) Get(asset Asset) Uint{
	ticker := asset.Ticker.String()
	if v, ok := balances[ticker]; ok {
		return v
	}
	return ZeroUint()
}
type Amounts struct {
	BaseAmount, QuoteAmount Uint
}
type PA struct {
	Price  Uint
	Amount Uint
}

// OfferSide - BID or ASK
var OfferSide = struct {
	BID int8
	ASK int8
}{int8(OrderSideBUY), int8(OrderSideSELL)}

type SwapTo int8
const (
	SwapToAsset  = SwapTo(0)
	SwapToQuote = SwapTo(1)
)
func (os SwapTo) String() string {if os == SwapToAsset { return "Swap To Asset" } else { return "Swap To RUNE" }}
func (os SwapTo) Invert() SwapTo {if os == SwapToAsset { return SwapToQuote } else { return SwapToAsset }}
func (os SwapTo) SrcAsset(market Market) Asset {if os == SwapToAsset { return market.QuoteAsset } else { return market.BaseAsset }}
func (os SwapTo) DstAsset(market Market) Asset {if os == SwapToAsset { return market.BaseAsset } else { return market.QuoteAsset }}
func (os SwapTo) DirStr(market Market) string {
	return os.SrcAsset(market).Ticker.String() + " -> " + os.DstAsset(market).Ticker.String()
	/* older implementation
	if os == SwapToAsset {
		return market.QuoteAsset.Ticker.String() + " -> " + market.BaseAsset.Ticker.String()
	} else {
		return market.BaseAsset.Ticker.String() + " -> " + market.QuoteAsset.Ticker.String()
	}
	*/
}

var OfferType = struct {
	ORDERBOOK int8
	TCPOOL    int8
}{0, 1}

func ZeroAmounts() Amounts { return Amounts{} }
func (a Amounts) Equal(a2 Amounts) bool {
	return a.BaseAmount.Equal(a2.BaseAmount) && a.QuoteAmount.Equal(a2.QuoteAmount)
}
func (a Amounts) IsEmpty() bool {
	return a.BaseAmount == ZeroUint() || a.QuoteAmount == ZeroUint()
}
func (a *Amounts) Flip() {
	a.BaseAmount, a.QuoteAmount = a.QuoteAmount, a.BaseAmount
}
	
var PA0 = PA{Price: ZeroUint(), Amount: ZeroUint()}

func (pa PA) String() string {
	if pa == PA0 {
		return "[zero]"
	}
	return fmt.Sprintf("[%s @ %s = %s]", pa.Amount, pa.Price, pa.Mul())
}
func (pa *PA) Str(asset1, asset2 string) string {
	if pa == nil {
		return "nil"
	}
	return fmt.Sprintf("%s %s @ %s = %s %s", pa.Amount, asset1, pa.Price, pa.Mul(), asset2)
}
func (pa *PA) Mul() Uint {
	if pa == nil {
		return ZeroUint()
	}
	return pa.Price.Mul(pa.Amount)
}

func (oes OrderbookEntries) Pr(i int) Uint {
	return oes[i].Price
}
func (oes OrderbookEntries) Am(i int) Uint {
	return oes[i].Amount
}
func (oes OrderbookEntries) Su(i int) Uint {
	sum := ZeroUint()
	for j := 0; j <= i; j++ {
		sum = sum.Add(oes[j].Amount)
	}
	return sum
}
func (oes OrderbookEntries) Am2(i int) Uint {
	return oes[i].Price.Mul(oes[i].Amount)
}
func (oes OrderbookEntries) Avg(amount Uint) (Uint, int) {
	if amount.IsZero() {
		return ZeroUint(), 1
	}
	debug := true
	avg := ZeroUint()
	sum := ZeroUint()
	for i := 0; i < len(oes); i++ {
		plus := oes.Am(i)
		if oes.Su(i).GT(amount) { //su(i) > am -> pl = am - sum
			plus = amount.Sub(sum)
		}
		if plus.IsZero() {
			if debug {
				log.Printf("Avg OB entry used: %v", i)
			}
			return avg, i
		}
		avg = avg.Mul(sum).Add(oes.Pr(i).Mul(plus)).Quo(sum.Add(plus))
		sum = sum.Add(plus)
		if oes.Su(i).GT(amount) {
			if debug {
				log.Printf("Avg OB entry used: %v", i+1)
			}
			return avg, i + 1
		}
	}
	if debug {
		log.Printf("Avg OB entry used: ALL = %v", len(oes))
	}
	return avg, len(oes)
}
func (oes OrderbookEntries) AmountForPrice(price Uint, tradeType OrderSide) Uint {
	debug := true
	comp := func(a, b Uint) bool {
		if tradeType == OrderSideBUY {
			return a.GTE(b)
		} else {
			return a.LTE(b)
		}
	}
	if comp(oes.Pr(0), price) {
		if debug {
			log.Printf("Amount For Price (%s): ZERO - no OB fit", price)
		}
		return ZeroUint()
	}
	for i := 0; i < len(oes); i++ {
		if comp(oes.Pr(i), price) {
			if debug {
				log.Printf("Amount For Price (%s): ENDING (using %v), sum: %s", price, i-1, oes.Su(i-1))
			}
			return oes.Su(i - 1)
		}
		if debug {
			log.Printf("Amount For Price (%s): %v-th OB fit (p: %s, a: %s), sum till now: %s", price, i, oes.Pr(i), oes.Am(i), oes.Su(i))
		}
	}
	if debug {
		log.Printf("Amount For Price (%s): FULL - all OB fit, sum: %s", price, oes.Su(len(oes)-1))
	}
	return oes.Su(len(oes) - 1)
}
func (oes OrderbookEntries) LimitPriceForAmount(amount Uint) Uint {
	for i := 0; i < len(oes); i++ {
		if oes.Su(i).GTE(amount) {
			return oes.Pr(i)
		}
	}
	return oes.Pr(len(oes) - 1)
}

type Result struct {
	Err         error
	PartialFill bool
	Amount      Uint
	QuoteAmount Uint
	AvgPrice    Uint
}
type DepositAddress struct {
	Address string
	Memo    string
}
type DepositAddresses map[string]DepositAddress
type WithdrawAddress string
type WithdrawAddresses map[string]WithdrawAddress
type OrderSide int8
const (
	OrderSideBUY  = OrderSide(0)
	OrderSideSELL = OrderSide(1)
)
func (os OrderSide) String() string {if os == OrderSideBUY { return "BUY" } else { return "SELL" }}
func (os OrderSide) Invert() OrderSide {if os == OrderSideBUY { return OrderSideSELL } else { return OrderSideBUY }}
func (os OrderSide) IfBuy(a, b Uint) Uint { if os == OrderSideBUY { return a } else { return b }}
func StrToFloat(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
func IfStr(c bool, a, b string) string { if c { return a } else { return b }}