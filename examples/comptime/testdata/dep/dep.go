// Package dep is imported by package importer.
package dep

// Bits is the size of a uint, a constant that depends on the target.
const Bits = 32 << (^uint(0) >> 63)

// Word is an array whose length depends on the target.
type Word [Bits / 8]byte

// state is a package-level variable.
var state int8

// Pure is comptime.
func Pure(x uint8) uint8 { return x + 1 }

// Impure reads a package-level variable.
func Impure() int8 { return hidden() }

// hidden reads a package-level variable.
func hidden() int8 { return state }

// Wide is comptime when host and target agree on the size of int.
func Wide(x int) int { return x * 2 }
