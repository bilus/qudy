// Package reasons holds an exported symbol for every reason a symbol is not comptime.
package reasons

import "unsafe"

// Variable is a package-level variable.
var Variable = 1

// Namer is an interface to call through.
type Namer interface{ Name() string }

// ReadsVariable reads a package-level variable.
func ReadsVariable() int { return Variable }

// ConstantOfVariable is a constant whose expression names a variable.
const ConstantOfVariable = len(table)

// table is an array, so its length is a constant.
var table [4]byte

// NoBody is implemented outside Go.
func NoBody(x int8) int8

// CallsNoBody calls a function that is not comptime.
func CallsNoBody(x int8) int8 { return NoBody(x) }

// CallsFunctionValue calls through a function value.
func CallsFunctionValue(f func() int8) int8 { return f() }

// CallsInterface calls through an interface.
func CallsInterface(n Namer) string { return n.Name() }

// FunctionValue uses a function as a value.
func FunctionValue() func(int8) int8 { return NoBody }

// Closure makes a function literal.
func Closure(x int8) func() int8 { return func() int8 { return x } }

// WritesParameter writes to memory reached through a parameter.
func WritesParameter(p *int8) { *p = 1 }

// WritesLoaded writes to memory it loaded from memory of its own.
func WritesLoaded(p *int8) {
	own := []*int8{p}
	*own[0] = 1
}

// Receives receives from a channel.
func Receives(c chan int8) int8 { return <-c }

// Starts starts a goroutine.
func Starts() { go CallsNoBody(1) }

// Recovers calls recover.
func Recovers() any { return recover() }

// RangesOverMap ranges over a map.
func RangesOverMap(m map[string]int8) int8 {
	var last int8
	for _, v := range m {
		last = v
	}
	return last
}

// UsesUnsafe converts a pointer to a number.
func UsesUnsafe(p *int8) uintptr { return uintptr(unsafe.Pointer(p)) }

// Float computes with a floating-point number.
func Float(x float64) float64 { return x * 2 }

// FloatField moves a floating-point number without arithmetic.
func FloatField(p *struct{ f float32 }) float32 { return p.f }

// Appends observes the capacity that append chose.
func Appends(s []int8) int { return cap(append(s, 1)) }

// StringToBytes observes the capacity of a conversion.
func StringToBytes(s string) int { return cap([]byte(s)) }

// Generic has a type parameter.
func Generic[T any](x T) T { return x }

// CallsGeneric calls a generic function.
func CallsGeneric(x int8) int8 { return Generic(x) }

// Asserts uses a construct no rule names.
func Asserts(x any) int8 { return x.(int8) }

// Defers uses a construct no rule names.
func Defers() int8 {
	defer CallsNoBody(1)
	return 1
}

// ComparesPointers compares values whose equality the language leaves open.
func ComparesPointers(a, b *struct{}) bool { return a == b }

// Prints calls a built-in that no rule names.
func Prints() { println("printed") }
