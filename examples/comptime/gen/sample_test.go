package gen

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

// Print the program of one seed, to read: COMPTIME_SAMPLE=7 go test -run Sample -v .
func TestSample(t *testing.T) {
	seed, err := strconv.Atoi(os.Getenv("COMPTIME_SAMPLE"))
	if err != nil {
		t.Skip("set COMPTIME_SAMPLE to a seed")
	}
	p := Example(seed, Default)
	fmt.Println(source(p))
	for _, s := range p.Symbols {
		fmt.Printf("%-10s %-14s %v %v\n", s.Label, key(s), s.Because, s.Layers)
	}
}
