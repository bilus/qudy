// Package accepted holds symbols that the first rules prove comptime.
package accepted

// Limit is a constant.
const Limit = 10

// Counter is a type with a method that only reads.
type Counter struct{ n uint8 }

// Value reads a field through the receiver.
func (c *Counter) Value() uint8 { return c.n }

// Add8 computes with a type of a fixed size.
func Add8(a, b uint8) uint8 { return a + b }

// Sum computes with int.
func Sum(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

// Must panics on a negative number.
func Must(x int8) int8 {
	if x < 0 {
		panic("negative")
	}
	return x
}

// NewCounter writes to memory it allocated.
func NewCounter() *Counter {
	c := &Counter{}
	c.n = Limit
	return c
}

// Table fills a map it made.
func Table() map[string]bool {
	m := map[string]bool{}
	m["a"] = true
	return m
}

// Greeting concatenates strings.
func Greeting(name string) string { return "hello, " + name }

// Clamp calls a function that is comptime.
func Clamp(x int8) int8 { return Must(x) }

// Big returns a constant that needs an int of 64 bits.
func Big() int { return 1 << 40 }

// First indexes with an int and computes nothing with it.
func First(s []uint8) uint8 { return s[0] }

// AllOnes is a constant with as many bits as a uint has.
const AllOnes = ^uint(0)

// Wide returns a constant that fits a uint of 32 bits and not an int of 32 bits.
func Wide() int { return 3_000_000_000 }

// The values of one expression under iota: only the first needs a uint of 64 bits.
const (
	Huge uint = 1 << (40 - 20*iota)
	Small
)
