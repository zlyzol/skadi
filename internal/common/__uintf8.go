package common
/*
import (
	"fmt"
	"strconv"
)

// Uint wraps integer with 256 bit range bound
// Checks overflow, underflow and division by zero
// Exists in range from 0 to 2^256-1
type Uint uint64

// NewUint constructs Uint from int64
func NewUint(n uint64) Uint {
	return Uint(n)
}

// NewUintFromString constructs Uint from string
func NewUintFromString(s string) Uint {
	u, err := ParseUint(s)
	if err != nil {
		panic(err)
	}
	return u
}

// ZeroUint returns unsigned zero.
func ZeroUint() Uint { return 0 }

// OneUint returns 1.
func OneUint() Uint { return OneUint8 }

// Uint64 converts Uint to uint64
// Panics if the value is out of range
func (u Uint) Uint64() uint64 {
	return uint64(u)
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
func (u Uint) Mul(u2 Uint) (res Uint) {
	return Uint((float64(u) * float64(u2)) / float64(OneUint8))
}

// Mul multiplies two Uints
func (u Uint) MulUint64(u2 uint64) (res Uint) { return Uint((float64(u) * float64(u2)) / float64(OneUint8)) }

// Quo divides Uint with Uint with Fixed8 precision
func (u Uint) Quo(u2 Uint) (res Uint) { 
	var fu float64 = float64(u)
	fu = fu * float64(OneUint8)
	fu = fu / float64(u2)
	res = Uint(fu)
	return res 
}

// Mod returns remainder after dividing with Uint
func (u Uint) Mod(u2 Uint) Uint {
	if u2.IsZero() {
		panic("division-by-zero")
	}
	return u % u2
}

// Incr increments the Uint by one.
func (u Uint) Incr() Uint {
	return u + OneUint8
}

// Decr decrements the Uint by one.
// Decr will panic if the Uint is zero.
func (u Uint) Decr() Uint {
	return u - OneUint8
}

// Quo divides Uint with uint64
func (u Uint) QuoUint64(u2 uint64) Uint { return u / Uint(u2) }

// Return the minimum of the Uints
func MinUint(u1, u2 Uint) Uint { if u1 < u2 { return u1 } else { return u2 } }

// Return the maximum of the Uints
func MaxUint(u1, u2 Uint) Uint { if u1 > u2 { return u1 } else { return u2 } }

// Human readable string
func (u Uint) _String() string { return u.String() }

// Human readable string
func (u Uint) String() string { 
	part1 := uint64(u / OneUint8)
	part2 := uint64(u - Uint(part1) * OneUint8)
	return fmt.Sprintf("%v.%08v", part1, part2)
}

// ParseUint reads a string-encoded Uint value and return a Uint.
func ParseUint(s string) (Uint, error) {
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot convert %s to Uint (err: %s)", s, err)
	}
	return Uint(u), nil
}
func Fixed8ToUint(f8 Fixed8) Uint {
	return NewUint(uint64(f8))
}
func UintToFixed8(u Uint) Fixed8 {
	return Fixed8(u.Uint64())
}
func FloatStringToUint(s string) Uint {
	parts := strings.SplitN(s, ".", 2)
	ip := NewUintFromString(parts[0])
	if len(parts) == 1 {
		return ip
	}
	i := 0
	for ; i < len(parts[1]); i++ {
		if parts[1][i] != '0' { break }
	}
	fp := ZeroUint()
	if i < len(parts[1]) {
		ptr := parts[1][i:]
		fp = NewUintFromString(ptr)
	}
	for i := len(parts[1]); i < precision; i++ {
		fp = fp.MulUint64(10)
	}
	return ip.MulUint64(uint64(Fixed8Decimals)).Add(fp)
}
func (u Uint) To8Float() float64 {
	return float64(u.Uint64()) / float64(Fixed8Decimals)
}

*/