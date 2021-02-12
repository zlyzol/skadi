package common

// Exchange interface
type Exchange interface {
	GetName() string
	GetAccount() Account
	GetTrader(market Market) Trader
	GetSwapper(market Market) Swapper
	GetMarkets() []Market
	UpdateLimits(limits Amounts, market Market, oppositeExchange Exchange, side OrderSide) Amounts
	Subscribe(Market, func(data interface{}))
	GetCurrentOfferData(Market) interface{}
	CanWait() bool // true if it is not necessary to make the trade immediately
}

// Account / Wallet interface
type Account interface {
	GetName() string
	GetBalances() Balances
	GetBalance(asset Asset) Uint
	Refresh() Balances
	GetDepositAddresses(asset Asset) DepositAddresses
	Send(amount Uint, asset Asset, target Account, wait bool) (txId *string, sent Uint, err error)
	WaitForDeposit(hash string) error
}

// Offer interface is used for combine Pool and/or Orderbook togerher
type Offer interface { 
	IsEmpty() bool
	GetMarket() Market
	GetExchange() Exchange
	Merge(Offer) (Offer, error)
	GetOrderbook() *Orderbook // return nil if not orderbook Offer
	GetPool() *Pool // return nil if not pool Offer
	UpdateCache()
}

// Trader interface
type Trader interface {
	Trade(side OrderSide, amount Uint, limit Uint) Order
}

// Swapper interface
type Swapper interface {
	//	Execute(trade int8, amount Uint, limit Uint) Order
	Swap(side SwapTo, amount Uint, limit Uint) Order
}
		
// Order interface
type Order interface {
	GetResult() Result
	Revert() error
	PartialRevert(amount Uint) error
}
// Pricer interface
type Pricer interface {
	GetRuneValueOf(amount Uint, asset Asset) Uint
	GetRunePriceOf(asset Asset) Uint
}
// SymbolSolver interface
type SymbolSolver interface {
	GetSymbolAsset(asset Asset) (Asset, error)
}
var Oracle Pricer
var Symbols SymbolSolver