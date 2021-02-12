package common

import (
	"fmt"
	"strings"
	"sort"
)

const (
	AllChainStr	= "***"
	UnknownChainStr = "???"
	BNBChainStr		= "BNB"
	ETHChainStr		= "ETH"
	BTCChainStr		= "BTC"
	THORChainStr	= "THOR"
)
var (
	BNBChain  = Chain([]string{BNBChainStr})
	ETHChain  = Chain([]string{ETHChainStr})
	BTCChain  = Chain([]string{BTCChainStr})
	THORChain = Chain([]string{THORChainStr})
	AllChain  = Chain([]string{AllChainStr})
	UnknownChain  = Chain([]string{UnknownChainStr})
	noChain = Chain([]string{})
)

type Chain []string

func NewChain(chain string) (Chain, error) {
	if chain == string(AllChainStr) {
		return noChain, fmt.Errorf("Chain Error: Cannot initialize multichan Chain with ***. Use NewMultiChain()")
	}
	if chain == string(UnknownChainStr) {
		return UnknownChain, nil
	}
	if len(chain) < 2 {
		return noChain, fmt.Errorf("Chain Error: Not enough characters: %s", chain)
	}

	if len(chain) > 10 {
		return noChain, fmt.Errorf("Chain Error: Too many characters: %s", chain)
	}
	return Chain{strings.ToUpper(chain)}, nil
}

func NewMultiChain(chains []string) (Chain, error) {
	for _, chain := range chains {
		if len(chain) < 2 {
			return noChain, fmt.Errorf("Chain Error: Not enough characters: %s", chain)
		}
		if len(chain) > 10 {
			return noChain, fmt.Errorf("Chain Error: Too many characters: %s", chain)
		}
	}
	sort.Strings(chains)
 	return Chain(chains), nil
}

func (c Chain) Equal(c2 Chain) bool {
	eq := len(c) == len(c2)
	if !eq { return false }
	for i := range c {
		eq = strings.EqualFold(c[i], c2[i])
		if !eq { return false }
	}
	return true
}

func (c Chain) IsEmpty() bool {
	return len(c) == 0 || (len(c) == 1 && strings.TrimSpace(c[0]) == "")
}

func (c Chain) IsUnknown() bool {
	return c.Equal(UnknownChain)
}

func (c Chain) String() string {
	// uppercasing again just incase someon created a ticker via Chain("rune")
	if len(c) == 0 { return UnknownChainStr }
	if len(c) == 1 { return strings.ToUpper(c[0]) }
	//return "***"
	return c.ChainString()
}

func (c Chain) ChainString() string {
	// uppercasing again just incase someon created a ticker via Chain("rune")
	if len(c) == 0 { return UnknownChainStr }
	s := strings.ToUpper(c[0])
	for i := 1; i < len(c); i++ {
		s += "|" + strings.ToUpper(c[i])
	}
	if len(c) > 1 { s = "(" + s + ")"}
	return s
}

func (c Chain) Contains(c2 Chain) bool {
	if len(c) == 0 || len(c2) == 0 {
		return false
	}
	if c.Equal(c2) {
		return true
	}
	if c.Equal(AllChain) {
		return true
	}
	if c.Equal(UnknownChain) || c2.Equal(UnknownChain) {
		return false
	}
	for _, slave := range c2 {
		found := false
		for _, master := range c {
			found = found || strings.EqualFold(master, slave)
			if found { break }
		}
		if !found {
			return false
		}
	}
	return true
}
func (c Chain) Intersects(c2 Chain) bool {
	if len(c) == 0 || len(c2) == 0 {
		return false
	}
	if c.Equal(c2) {
		return true
	}
	if c.Equal(AllChain) || c2.Equal(AllChain) {
		return true
	}
	if c.Equal(UnknownChain) || c2.Equal(UnknownChain) {
		return false
	}
	for _, chain1 := range c2 {
		for _, chain2 := range c {
			if strings.EqualFold(chain1, chain2) {
				return true
			}
		}
	}
	return false
}