package gen

import (
	"fmt"
	"regexp"
	"testing"

	"pgregory.net/rapid"
)

// Why do some victims not change between runs? Split them by result type.
func TestVictimsByType(t *testing.T) {
	if testing.Short() {
		t.Skip("builds sixty binaries")
	}
	dir := t.TempDir()
	planted := rapid.Custom(func(t *rapid.T) Program { return Generate(t, true) })
	changed, total := map[string]int{}, map[string]int{}
	for seed, built := 0, 0; built < 60; seed++ {
		p := planted.Example(seed)
		if p.Impurity != "clock" && p.Impurity != "random number" {
			continue
		}
		built++
		typ := regexp.MustCompile(`func ` + p.Victim + `\([^)]*\) (\w+) \{`).FindStringSubmatch(p.Src)[1]
		bin := build(t, dir, fmt.Sprintf("bt%d", seed), p.Src, "GOARCH=amd64")
		same := true
		first := lines(run(bin))[p.Victim]
		for range 6 {
			same = same && lines(run(bin))[p.Victim] == first
		}
		total[typ]++
		if !same {
			changed[typ]++
		}
	}
	for _, typ := range []string{"int64", "bool", "string"} {
		t.Logf("victims returning %-6s: %d of %d changed within seven runs", typ, changed[typ], total[typ])
	}
}
