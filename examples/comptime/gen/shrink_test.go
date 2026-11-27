package gen

import (
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// naive stands in for an analyzer with one hole: it follows every call except one through an interface.
func naive(p Program) (comptime map[string]bool) {
	comptime = map[string]bool{}
	hidden := slices.Contains(p.Layers, "interface method")
	for _, name := range p.Funcs {
		comptime[name] = !slices.Contains(p.Tainted, name) || hidden
	}
	return comptime
}

// How small is the counterexample rapid reports for that hole? Expected to fail.
func TestShrinkDemo(t *testing.T) {
	if os.Getenv("COMPTIME_EXPERIMENTS") == "" {
		t.Skip("set COMPTIME_EXPERIMENTS=1; the test fails by design")
	}
	rapid.Check(t, func(t *rapid.T) {
		p := Generate(t, true)
		if naive(p)[p.Victim] {
			src, _ := format.Source([]byte(p.Src))
			t.Fatalf("%s is listed as comptime, and its result depends on: %s behind %v\nLINES %d\n%s", p.Victim, p.Impurity, p.Layers, strings.Count(string(src), "\n"), src)
		}
	})
}

// Can more runs, or another GOMAXPROCS, show the goroutine impurity that five oracles missed?
func TestSchedulingVisibility(t *testing.T) {
	if os.Getenv("COMPTIME_EXPERIMENTS") == "" {
		t.Skip("set COMPTIME_EXPERIMENTS=1; runs a binary 700 times")
	}
	var imp impurity
	for _, i := range impurities {
		if i.name == "goroutine scheduling" {
			imp = i
		}
	}
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc impure() int64 {\n\t" + imp.body + "\n}\n\nfunc main() {\n\tfmt.Println(impure())\n}\n"
	dir := t.TempDir()
	bin := build(t, dir, "sched", src, "GOARCH=amd64")
	for _, procs := range []string{"1", "4"} {
		seen := map[string]int{}
		for range 300 {
			seen[strings.TrimSpace(run(bin, "GOMAXPROCS="+procs))]++
		}
		t.Logf("GOMAXPROCS=%s, 300 runs: %v", procs, seen)
	}
	race := filepath.Join(dir, "sched.race")
	if out, err := exec.Command("go", "build", "-race", "-o", race, filepath.Join(dir, "sched.go")).CombinedOutput(); err != nil {
		t.Logf("no race build here: %v %s", err, firstLine(string(out)))
		return
	}
	seen := map[string]int{}
	for range 100 {
		seen[strings.TrimSpace(run(race))]++
	}
	t.Logf("-race, 100 runs: %v", seen)
}

// firstLine returns the first line of a text.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return fmt.Sprint(line)
}
