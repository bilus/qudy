// Package cgo holds symbols of a file that cgo rewrites.
package cgo

// #define ANSWER 42
import "C"

// Answer is a constant that comes from C.
const Answer = C.ANSWER

// Plain is Go code in a file that imports C.
func Plain(x int8) int8 { return x }
