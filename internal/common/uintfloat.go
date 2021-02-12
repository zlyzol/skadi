package common

import (
	"fmt"
	"math"
	"strconv"
	types "github.com/binance-chain/go-sdk/common/types"
)

const (
	precision = 8
)
var (
	// Fixed8Decimals represents 10^precision (100000000), a value of 1 in Fixed8 format
	Fixed8Decimals = uint64(math.Pow10(precision))
	Fixed8DecimalsUint = NewUint(Fixed8Decimals)
	MaxUintValue = NewUint(math.MaxUint64)
)

// Uint wraps integer with 256 bit range bound
// Checks overflow, underflow and division by zero
// Exists in range from 0 to 2^256-1
type Uint float64

// NewUint constructs Uint from int64
func NewUint(n uint64) Uint {
	return Uint(n)
}
func NewUintFromString(s string) Uint {
	u, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Errorf("cannot convert %s to Uint (err: %s)", s, err))
	}
	return Uint(u)
}
func NewUintFromFloat(f float64) Uint {
	return Uint(f)
}
func NewUintFromFx8(f8 types.Fixed8) Uint {
	return Uint(Uint(f8) / Fixed8DecimalsUint)
}
func NewUintFromFx8String(f8s string) Uint {
	u, err := ParseUint(f8s)
	if err != nil {
		return ZeroUint()
	}
	return u / Fixed8DecimalsUint
}

// ZeroUint returns unsigned zero.
func ZeroUint() Uint { return 0 }

// OneUint returns 1.
func OneUint() Uint { return 1 }

// OneUint returns 1 * 10^-8.
func OneFx8Uint() Uint { return Uint(float64(1.0) / float64(Fixed8Decimals)) }

// Uint64 converts Uint to uint64
func (u Uint) Fx8Uint64() uint64 {
	return uint64(u)
}

// Int64 converts Uint to Int64
func (u Uint) Fx8Int64() int64 {
	return int64(math.Round((u * Fixed8DecimalsUint).ToFloat()))
}

// IsZero returns 1 if the uint equals to 0.
func (u Uint) IsZero() bool { return u == 0 }

// Equal compares two Uints
func (u Uint) Equal(u2 Uint) bool { return u == u2 }

// GT returns true if first Uint is greater than second
func (u Uint) GT(u2 Uint) bool { return u > u2 }

// GTE returns true if first Uint is greater than second
func (u Uint) GTE(u2 Uint) bool { return u >= u2 }

// LT returns true if first Uint is lesser than second
func (u Uint) LT(u2 Uint) bool { return u < u2 }

// LTE returns true if first Uint is lesser than or equal to the second
func (u Uint) LTE(u2 Uint) bool { return u <= u2 }

// Add adds Uint from another
func (u Uint) Add(u2 Uint) Uint { return u + u2 }

// Add convert uint64 and add it to Uint
func (u Uint) AddUint64(u2 uint64) Uint { return u + Uint(u2) }

// Sub adds Uint from another
func (u Uint) Sub(u2 Uint) Uint { return u - Uint(u2) }

// SubUint64 adds Uint from another
func (u Uint) SubUint64(u2 uint64) Uint { return u - Uint(u2) }

// Mul multiplies two Uints
func (u Uint) Mul(u2 Uint) (res Uint) { return roundFx8(u * u2) }

// Mul multiplies two Uints
func (u Uint) MulUint64(u2 uint64) (res Uint) { return roundFx8(u * Uint(u2)) }

// Quo divides Uint with Uint with Fixed8 precision
func (u Uint) Quo(u2 Uint) (res Uint) { 
	return roundFx8(u / u2)
}

// Incr increments the Uint by one.
func (u Uint) Incr() Uint {
	return u + 1
}

// Decr decrements the Uint by one.
// Decr will panic if the Uint is zero.
func (u Uint) Decr() Uint {
	return u - 1
}

// Quo divides Uint with uint64
func (u Uint) QuoUint64(u2 uint64) Uint { return roundFx8(u / Uint(u2)) }

// Return the minimum of the Uints
func MinUint(u1, u2 Uint) Uint { if u1 < u2 { return u1 } else { return u2 } }

// Return the maximum of the Uints
func MaxUint(u1, u2 Uint) Uint { if u1 > u2 { return u1 } else { return u2 } }

// Human readable string
func (u Uint) _String() string { return u.String() }

// Fixed8 style floor
func (u Uint) Floor() Uint { 
	return Uint(math.Floor(u.ToFloat()))
}

// Human readable string
func (u Uint) String() string { 
	return fmt.Sprintf("%0.08f", u)
}
func (u Uint) ToFx8String() string { 
	return strconv.FormatUint(uint64(u), 10)
}

// ParseUint reads a string-encoded Uint value and return a Uint.
func ParseUint(s string) (Uint, error) {
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return ZeroUint(), fmt.Errorf("cannot convert %s to Uint (err: %s)", s, err)
	}
	return Uint(u), nil
}
func (u Uint) ToFixed8() types.Fixed8 {
	return types.Fixed8(u * Fixed8DecimalsUint)
}
func (u Uint) ToFloat() float64 {
	return float64(u)
}
func (u Uint) ToFx8Float() float64 {
	return float64(u * Fixed8DecimalsUint)
}
func roundFx8(u Uint) Uint {
	return Uint(math.Round(float64(u) * float64(Fixed8DecimalsUint)) / float64(Fixed8DecimalsUint))
}
func (u Uint) RoundFx8() Uint {
	return roundFx8(u)
}
func (u Uint) RoundTo(mul Uint) Uint {
	if mul.IsZero() { return u }
	return Uint(math.Round(u.ToFx8Float() / mul.ToFx8Float()) * mul.ToFx8Float() / float64(Fixed8DecimalsUint))
}