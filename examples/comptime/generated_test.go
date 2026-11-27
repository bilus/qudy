package comptime_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/comptime"
	"github.com/bilus/qudy/examples/comptime/gen"
)

// excused maps an impurity to the condition under which its function is comptime after all.
// The value depends on the target only, and the condition says that host and target agree.
var excused = map[string]comptime.Condition{
	"size of int":            comptime.SameIntSize,
	"constant of the target": comptime.SameBuildConfig,
}

// generate writes n programs into one module below a new directory, and returns them.
// Two of every three have impurities, and one has none.
func generate(t *testing.T, n int, impure, pure gen.Config) (dir string, programs []gen.Program) {
	t.Helper()
	dir = t.TempDir()
	write := func(name, src string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/generated\n\ngo 1.25\n")
	for seed := range n {
		cfg := impure
		if seed%3 == 2 {
			cfg = pure
		}
		cfg.Module = fmt.Sprintf("example.com/generated/prog%d", seed)
		p := gen.Example(seed, cfg)
		for _, k := range p.Packages {
			write(filepath.Join(fmt.Sprintf("prog%d", seed), k.File), k.Src)
		}
		programs = append(programs, p)
	}
	return dir, programs
}

// outcome compares the analyzer's verdicts with the generator's labels.
type outcome struct {
	impure, comptime, listed, unknown, unknownListed int
	wrong                                            []string                  // the functions with a planted impurity that are listed
	misses                                           map[string]int            // why a comptime function is not listed
	caught                                           map[string]map[string]int // impurity, then the kind of reason at the end of the chain
}

// compare analyzes the programs under the rules and compares verdicts with labels.
func compare(t *testing.T, dir string, programs []gen.Program, rules comptime.Rules) outcome {
	t.Helper()
	report, err := comptime.Analyze(comptime.Config{Dir: dir, Rules: rules}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range report.Packages {
		if pkg.Error != "" {
			t.Fatalf("%s was not analyzed: %s", pkg.Path, pkg.Error)
		}
	}
	o := outcome{misses: map[string]int{}, caught: map[string]map[string]int{}}
	for _, p := range programs {
		for _, s := range p.Symbols {
			if !s.Exported {
				continue
			}
			sym, ok := lookup(report, s.Package, s.Name)
			if !ok {
				t.Fatalf("the report has no symbol %s.%s", s.Package, s.Name)
			}
			switch s.Label {
			case gen.Impure:
				o.impure++
				if o.caught[s.Impurity] == nil {
					o.caught[s.Impurity] = map[string]int{}
				}
				c, canExcuse := excused[s.Impurity]
				switch {
				case !sym.Comptime:
					o.caught[s.Impurity][string(sym.Reason.Root().Kind)]++
				case canExcuse && slices.Contains(sym.Conditions, c):
					o.caught[s.Impurity]["condition "+string(c)]++
				default:
					o.wrong = append(o.wrong, fmt.Sprintf("%s.%s is listed as %v, and its result depends on %q behind %v", s.Package, s.Name, sym.Verdict, s.Impurity, s.Layers))
				}
			case gen.Comptime:
				o.comptime++
				if sym.Comptime {
					o.listed++
					continue
				}
				root := sym.Reason.Root()
				o.misses[string(root.Kind)+": "+root.Detail]++
			case gen.Unknown:
				o.unknown++
				if sym.Comptime {
					o.unknownListed++
				}
			}
		}
	}
	return o
}

// The analyzer never lists a generated function with a planted impurity. The test also
// reports how many of the functions that are comptime by construction it lists.
func TestGeneratedPrograms(t *testing.T) {
	if testing.Short() {
		t.Skip("analyzes generated programs and the packages they import")
	}
	dir, programs := generate(t, 150, gen.Default, gen.Pure)
	o := compare(t, dir, programs, comptime.FirstRules{})
	for _, w := range o.wrong {
		t.Error(w)
	}
	t.Logf("%d programs", len(programs))
	t.Logf("functions with a planted impurity: %d, of which %d are listed without the condition that excuses them", o.impure, len(o.wrong))
	t.Logf("functions that are comptime by construction: %d listed of %d", o.listed, o.comptime)
	t.Logf("functions labelled unknown: %d listed of %d", o.unknownListed, o.unknown)
	t.Logf("why a comptime function was not listed, by the construct at the end of the chain:\n%s", counts(o.misses))
	var rows []string
	for impurity, kinds := range o.caught {
		rows = append(rows, fmt.Sprintf("  %-30s %s", impurity, strings.Join(strings.Fields(counts(kinds)), " ")))
	}
	sort.Strings(rows)
	t.Logf("what kept each planted impurity off the list:\n%s", strings.Join(rows, "\n"))
	if o.impure == 0 || o.comptime == 0 {
		t.Errorf("the programs hold %d impure and %d comptime functions, want many of each", o.impure, o.comptime)
	}
}

// The first rules cover the core of the generator's fragment: every function that is
// comptime by construction is listed, unless it holds a construct on the list below.
func TestCorePrograms(t *testing.T) {
	if testing.Short() {
		t.Skip("analyzes generated programs and the packages they import")
	}
	dir, programs := generate(t, 150, gen.Core, gen.CorePure)
	o := compare(t, dir, programs, comptime.FirstRules{})
	for _, w := range o.wrong {
		t.Error(w)
	}
	t.Logf("functions with a planted impurity: %d, of which %d are listed", o.impure, len(o.wrong))
	t.Logf("functions that are comptime by construction: %d listed of %d", o.listed, o.comptime)
	t.Logf("why a comptime function was not listed:\n%s", counts(o.misses))
	for miss, n := range o.misses {
		if !knownGaps[miss] {
			t.Errorf("%d comptime functions were not listed for a reason that is not a known gap: %s", n, miss)
		}
	}
}

// knownGaps are the constructs of the core fragment that the first rules reject.
// Each is comptime in the generated programs, and none is proven so by a rule yet.
var knownGaps = map[string]bool{}

// without is the first rule set with one rule disabled: a rejection whose chain ends in
// the reason kind counts as comptime.
type without struct {
	comptime.FirstRules
	kind comptime.ReasonKind
}

// pass turns a rejection for the disabled reason into an acceptance.
func (w without) pass(v comptime.Verdict) comptime.Verdict {
	if !v.Comptime && v.Reason.Root().Kind == w.kind {
		return comptime.Accepted()
	}
	return v
}

// Const judges a constant with the rule disabled.
func (w without) Const(c *comptime.Constant, look comptime.Lookup) comptime.Verdict {
	return w.pass(w.FirstRules.Const(c, look))
}

// Var judges a variable with the rule disabled.
func (w without) Var(v *comptime.Variable) comptime.Verdict { return w.pass(w.FirstRules.Var(v)) }

// Type judges a type with the rule disabled.
func (w without) Type(d *comptime.TypeDecl, look comptime.Lookup) comptime.Verdict {
	return w.pass(w.FirstRules.Type(d, look))
}

// Func judges a function with the rule disabled.
func (w without) Func(f *comptime.Function, look comptime.Lookup) comptime.Verdict {
	return w.pass(w.FirstRules.Func(f, look))
}

// tested are the rules whose removal must list a function with a planted impurity.
// The generator plants few or no impurities that only one of the other rules catches.
var tested = map[comptime.ReasonKind]bool{
	comptime.ReasonVariable:     true,
	comptime.ReasonNoBody:       true,
	comptime.ReasonDynamicCall:  true,
	comptime.ReasonFuncValue:    true,
	comptime.ReasonForeignWrite: true,
	comptime.ReasonConcurrency:  true,
	comptime.ReasonMapRange:     true,
	comptime.ReasonFloat:        true,
	comptime.ReasonUnspecified:  true,
}

// The tests above can fail: with one of the tested rules disabled, the analyzer lists a
// function with a planted impurity. The programs are core programs, because in the
// others a call of a trusted symbol, which the first rules reject, hides the disabled rule.
func TestDisabledRules(t *testing.T) {
	if testing.Short() {
		t.Skip("analyzes generated programs once for every reason kind")
	}
	dir, programs := generate(t, 120, gen.Core, gen.CorePure)
	var untested []string
	for _, kind := range comptime.ReasonKinds() {
		wrong := len(compare(t, dir, programs, without{kind: kind}).wrong)
		switch {
		case wrong > 0:
			t.Logf("without %-22q %d functions with a planted impurity are listed", kind, wrong)
		case tested[kind]:
			t.Errorf("without the rule %q no function with a planted impurity is listed", kind)
		default:
			untested = append(untested, string(kind))
		}
	}
	t.Logf("rules whose removal lists no planted impurity: %s", strings.Join(untested, ", "))
}

// counts formats counts, largest first.
func counts(m map[string]int) string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if m[names[i]] != m[names[j]] {
			return m[names[i]] > m[names[j]]
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "  %4d  %s\n", m[name], name)
	}
	return b.String()
}
