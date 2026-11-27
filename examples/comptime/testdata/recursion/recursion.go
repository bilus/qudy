// Package recursion holds functions that call themselves and each other.
package recursion

// Factorial calls itself.
func Factorial(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	return n * Factorial(n-1)
}

// Even and Odd call each other.
func Even(n uint8) bool {
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

// Odd and Even call each other.
func Odd(n uint8) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

// Tainted calls Carrier, and then writes through its parameter.
func Tainted(n uint8, p *uint8) uint8 {
	if n == 0 {
		return 0
	}
	r := Carrier(n-1, p)
	*p = r
	return r
}

// Carrier is clean itself, and not comptime because it calls Tainted.
func Carrier(n uint8, p *uint8) uint8 { return Tainted(n, p) }

// Sized calls Unsized, and computes with int.
func Sized(n uint8) int {
	if n == 0 {
		return 0
	}
	return Unsized(n-1) + 1
}

// Unsized computes nothing with int itself, and takes the condition from Sized.
func Unsized(n uint8) int {
	if n == 0 {
		return 0
	}
	return Sized(n - 1)
}

// List is a type that names itself.
type List struct {
	next *List
	head uint8
}

// Head reads through a type that names itself.
func (l *List) Head() uint8 { return l.head }
