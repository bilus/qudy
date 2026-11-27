// Package importer takes its verdicts from those of package dep.
package importer

import "github.com/bilus/qudy/examples/comptime/testdata/dep"

// UsesPure calls a comptime function of a dependency.
func UsesPure(x uint8) uint8 { return dep.Pure(x) }

// UsesImpure calls a function of a dependency that is not comptime.
func UsesImpure() int8 { return dep.Impure() }

// UsesWide inherits a condition from a function of a dependency.
func UsesWide(x uint8) uint8 { return uint8(dep.Wide(1)) + x }

// UsesBits inherits a condition from a constant of a dependency.
const UsesBits = dep.Bits / 8

// UsesWord inherits a condition from a type of a dependency.
func UsesWord() bool {
	var w dep.Word
	return len(w) == 8
}
