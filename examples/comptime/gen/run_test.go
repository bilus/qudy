package gen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// conditions are the ways two executions of one program can differ.
var conditions = []string{"repeat", "rerun", "env", "v3", "386"}

// targets are the builds the conditions need.
var targets = map[string][]string{
	"amd64": {"GOARCH=amd64", "GOAMD64=v1"},
	"v3":    {"GOARCH=amd64", "GOAMD64=v3"},
	"386":   {"GOARCH=386"},
}

// build compiles a program for every target and returns the binaries.
func build(t *testing.T, p Program) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := p.Write(dir); err != nil {
		t.Fatal(err)
	}
	bins := map[string]string{}
	for name, env := range targets {
		bins[name] = filepath.Join(dir, "bin-"+name)
		cmd := exec.Command("go", "build", "-o", bins[name], ".")
		cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build for %s: %v\n%s\n%s", name, err, out, source(p))
		}
	}
	return bins
}

// source returns every file of a program.
func source(p Program) string {
	var b strings.Builder
	for _, k := range p.Packages {
		fmt.Fprintf(&b, "// %s\n%s\n", k.File, k.Src)
	}
	return b.String()
}

// run executes a binary and returns, for each of the two rounds, the line every function printed.
func run(t *testing.T, bin string, env ...string) [2]map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env, cmd.Dir = append(os.Environ(), env...), os.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	halves := strings.SplitN(string(out), "--\n", 2)
	if len(halves) != 2 {
		t.Fatalf("the program printed no second round:\n%s", out)
	}
	var rounds [2]map[string]string
	for i, half := range halves {
		rounds[i] = map[string]string{}
		for _, line := range strings.Split(half, "\n") {
			if name, value, ok := strings.Cut(line, " "); ok {
				rounds[i][name] = value
			}
		}
	}
	return rounds
}

// observation is what the conditions showed for the functions of one program.
type observation struct {
	differs map[string]map[string]bool // condition, then function
	panics  map[string]bool
	calls   int
}

// observe builds a program, runs it under every condition, and compares the results per function.
func observe(t *testing.T, p Program) observation {
	t.Helper()
	bins := build(t, p)
	base := run(t, bins["amd64"])
	others := map[string]map[string]string{
		"repeat": base[1],
		"rerun":  run(t, bins["amd64"])[0],
		"env":    run(t, bins["amd64"], "HOME=/nonexistent/elsewhere", "TZ=Asia/Tokyo")[0],
		"v3":     run(t, bins["v3"])[0],
		"386":    run(t, bins["386"])[0],
	}
	o := observation{differs: map[string]map[string]bool{}, panics: map[string]bool{}, calls: len(base[0])}
	for _, c := range conditions {
		o.differs[c] = map[string]bool{}
		for name, value := range base[0] {
			o.differs[c][name] = others[c][name] != value
		}
	}
	for name, value := range base[0] {
		o.panics[name] = strings.HasPrefix(value, "panic: ")
	}
	return o
}

// key returns the name a program prints for a symbol.
func key(s Symbol) string { return path.Base(s.Package) + "." + s.Name }

// any reports the conditions under which a function's result differed.
func (o observation) any(name string) (seen []string) {
	for _, c := range conditions {
		if o.differs[c][name] {
			seen = append(seen, c)
		}
	}
	return seen
}

// count returns the number of programs a test builds, which COMPTIME_PROGRAMS overrides.
func count(def int) int {
	if n, err := strconv.Atoi(os.Getenv("COMPTIME_PROGRAMS")); err == nil {
		return n
	}
	return def
}

// The labels hold when the programs run: a Comptime function gives one result under every
// condition, and an Impure function gives two under the condition its impurity names.
func TestLabelsHoldAtRunTime(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs generated programs")
	}
	type tally struct{ ok, total int }
	var comptime, panicked, pure tally
	shown, quiet := map[string]*tally{}, map[string]*tally{}
	check := func(seed int, cfg Config) {
		p := Example(seed, cfg)
		o := observe(t, p)
		for _, s := range p.Symbols {
			if !s.Exported {
				continue
			}
			seen := o.any(key(s))
			switch {
			case s.Label == Comptime:
				count := &comptime
				if cfg.Impurities == 0 {
					count = &pure
				}
				count.total++
				if o.panics[key(s)] {
					panicked.total++
				}
				if len(seen) > 0 {
					t.Errorf("seed %d: %s is labelled comptime, and its result differs under %v\n%s", seed, key(s), seen, source(p))
					continue
				}
				count.ok++
			case s.Label == Impure && s.Shows != "":
				c := shown[s.Impurity]
				if c == nil {
					c = &tally{}
					shown[s.Impurity] = c
				}
				c.total++
				if !o.differs[s.Shows][key(s)] {
					t.Errorf("seed %d: %s depends on %q behind %v, and its result is the same under %q\n%s", seed, key(s), s.Impurity, s.Layers, s.Shows, source(p))
					continue
				}
				c.ok++
			case s.Label == Impure:
				c := quiet[s.Impurity]
				if c == nil {
					c = &tally{}
					quiet[s.Impurity] = c
				}
				c.total++
				if len(seen) > 0 {
					c.ok++
				}
			}
		}
	}
	n := count(24)
	for seed := range n {
		check(seed, Default)
	}
	for seed := range n / 2 {
		check(seed, Pure)
	}
	t.Logf("%d programs with impurities, %d without", n, n/2)
	t.Logf("comptime functions with one result under all five conditions: %d of %d, and %d of %d in programs without impurities", comptime.ok, comptime.total, pure.ok, pure.total)
	t.Logf("comptime functions whose call panicked, with the same value each time: %d", panicked.total)
	var names []string
	for name := range shown {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("  %-32s shown by its condition in %d of %d functions", name, shown[name].ok, shown[name].total)
	}
	for name, c := range quiet {
		t.Logf("  %-32s has no reliable condition; some condition showed it in %d of %d functions", name, c.ok, c.total)
	}
}

// planted returns a small program whose one impurity is the catalog entry i, with an exported function.
func planted(i int) (Program, Symbol) {
	for seed := 0; ; seed++ {
		p := rapid.Custom(func(t *rapid.T) Program {
			m := newMachine(Config{Rounds: 1, Packages: 1, Impurities: 1})
			m.only = i
			t.Repeat(m.actions())
			return m.finish(t)
		}).Example(seed)
		for _, s := range p.Symbols {
			if s.Label == Impure && s.Exported {
				return p, s
			}
		}
	}
}

// Every catalog entry, alone in a program, is exposed by the condition it names and by no cheaper one.
func TestEveryImpurityShows(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs generated programs")
	}
	var rows []string
	for i, imp := range impurities {
		p, s := planted(i)
		o := observe(t, p)
		seen := o.any(key(s))
		if imp.shows != "" && !o.differs[imp.shows][key(s)] {
			t.Errorf("%q is the same under %q:\n%s", imp.name, imp.shows, source(p))
		}
		for _, other := range p.Symbols {
			if other.Label == Comptime && other.Exported && len(o.any(key(other))) > 0 {
				t.Errorf("%s is comptime and differs under %v:\n%s", key(other), o.any(key(other)), source(p))
			}
		}
		expected := imp.shows
		if expected == "" {
			expected = "(none)"
		}
		rows = append(rows, fmt.Sprintf("  %-30s expected %-7s observed %v", imp.name, expected, seen))
	}
	t.Log("\n" + strings.Join(rows, "\n"))
}
