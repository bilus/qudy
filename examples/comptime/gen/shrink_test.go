package gen

import (
	"os"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

// How small is the counterexample rapid reports for an analyzer with one hole?
// The stand-in analyzer misses an impurity behind a call through an interface.
// The test fails by design: COMPTIME_SHRINK=1 go test -run ShrinkDemo -v .
func TestShrinkDemo(t *testing.T) {
	if os.Getenv("COMPTIME_SHRINK") == "" {
		t.Skip("set COMPTIME_SHRINK=1; the test fails by design")
	}
	rapid.Check(t, func(t *rapid.T) {
		p := Generate(t, Default)
		for _, s := range p.Symbols {
			if s.Label == Impure && slices.Contains(s.Layers, "interface method") {
				t.Fatalf("%s is listed as comptime, and depends on %q behind %v\nLINES %d FUNCTIONS %d\n%s",
					key(s), s.Impurity, s.Layers, lines(p), len(p.Symbols), source(p))
			}
		}
	})
}
