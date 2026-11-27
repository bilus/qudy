package comptime_test

import (
	"bytes"
	"go/build"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/comptime"
)

// testdata is the import path of the directory that holds the test packages.
const testdata = "github.com/bilus/qudy/examples/comptime/testdata/"

// analyze judges test packages, named by their directory under testdata.
func analyze(t *testing.T, cfg comptime.Config, names ...string) *comptime.Report {
	t.Helper()
	patterns := make([]string, len(names))
	for i, name := range names {
		patterns[i] = testdata + name
	}
	report, err := comptime.Analyze(cfg, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range report.Packages {
		if pkg.Error != "" {
			t.Fatalf("%s was not analyzed: %s", pkg.Path, pkg.Error)
		}
	}
	return report
}

// lookup returns the symbol of a report with the given package path and name.
func lookup(report *comptime.Report, path, name string) (comptime.Symbol, bool) {
	for _, pkg := range report.Packages {
		if pkg.Path != path {
			continue
		}
		for _, sym := range pkg.Symbols {
			if sym.Name == name {
				return sym, true
			}
		}
	}
	return comptime.Symbol{}, false
}

// symbol returns a symbol of a test package, named as package.Name.
func symbol(t *testing.T, report *comptime.Report, name string) comptime.Symbol {
	t.Helper()
	pkg, rest, _ := strings.Cut(name, ".")
	sym, ok := lookup(report, testdata+pkg, rest)
	if !ok {
		t.Fatalf("the report has no symbol %s", name)
	}
	return sym
}

// needCgo skips a test on a machine that cannot build a package that imports C.
func needCgo(t *testing.T) {
	t.Helper()
	if !build.Default.CgoEnabled {
		t.Skip("cgo is disabled")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("there is no C compiler")
	}
}

func TestAnalyzeAcceptsWhatTheRulesProve(t *testing.T) {
	report := analyze(t, comptime.Config{}, "accepted")
	type testCase struct {
		description string
		symbol      string
		kind        comptime.Kind
		conditions  []comptime.Condition
	}
	for _, c := range []testCase{
		{"a constant", "accepted.Limit", comptime.KindConst, nil},
		{"arithmetic on a type of a fixed size", "accepted.Add8", comptime.KindFunc, nil},
		{"arithmetic on int holds for one size of int", "accepted.Sum", comptime.KindFunc, []comptime.Condition{comptime.SameIntSize}},
		{"an explicit panic", "accepted.Must", comptime.KindFunc, nil},
		{"a write to memory the function allocated", "accepted.NewCounter", comptime.KindFunc, nil},
		{"an update of a map the function made", "accepted.Table", comptime.KindFunc, nil},
		{"a concatenation is as long as the int of the target allows", "accepted.Greeting", comptime.KindFunc, []comptime.Condition{comptime.SameIntSize}},
		{"a call of a comptime function", "accepted.Clamp", comptime.KindFunc, nil},
		{"a method that reads through its receiver", "accepted.Counter.Value", comptime.KindMethod, nil},
		{"a constant that needs 64 bits holds for one size of int", "accepted.Big", comptime.KindFunc, []comptime.Condition{comptime.SameIntSize}},
		{"a constant between the largest int32 and uint32 does, too", "accepted.Wide", comptime.KindFunc, []comptime.Condition{comptime.SameIntSize}},
		{"an index of type int is the same number on every target", "accepted.First", comptime.KindFunc, nil},
		{"a complement of a uint has as many bits as the target", "accepted.AllOnes", comptime.KindConst, []comptime.Condition{comptime.SameIntSize}},
		{"a constant under iota is judged by its own value", "accepted.Huge", comptime.KindConst, []comptime.Condition{comptime.SameIntSize}},
		{"a constant of a file under a build constraint", "accepted.Tagged", comptime.KindConst, []comptime.Condition{comptime.SameBuildConfig}},
		{"a function of such a file", "accepted.UsesTagged", comptime.KindFunc, []comptime.Condition{comptime.SameBuildConfig}},
	} {
		t.Run(c.description, func(t *testing.T) {
			sym := symbol(t, report, c.symbol)
			if sym.Kind != c.kind {
				t.Errorf("%s is a %s, want a %s", c.symbol, sym.Kind, c.kind)
			}
			if !sym.Comptime || sym.Reason != nil || !slices.Equal(sym.Conditions, c.conditions) {
				t.Errorf("%s is %v with conditions %v, want comptime with %v", c.symbol, sym.Verdict, sym.Conditions, c.conditions)
			}
		})
	}
}

func TestAnalyzeGivesAReasonOfEveryKind(t *testing.T) {
	report := analyze(t, comptime.Config{}, "reasons")
	type testCase struct {
		description string
		symbol      string
		kind        comptime.ReasonKind
		cause       string // the symbol the reason names, without the path of the package
		root        comptime.ReasonKind
	}
	cases := []testCase{
		{"a package-level variable", "reasons.Variable", comptime.ReasonVariable, "", comptime.ReasonVariable},
		{"a read of a package-level variable", "reasons.ReadsVariable", comptime.ReasonVariable, "reasons.Variable", comptime.ReasonVariable},
		{"a constant that names a variable", "reasons.ConstantOfVariable", comptime.ReasonVariable, "reasons.table", comptime.ReasonVariable},
		{"a function without a body", "reasons.NoBody", comptime.ReasonNoBody, "", comptime.ReasonNoBody},
		{"a call of a function that is not comptime", "reasons.CallsNoBody", comptime.ReasonDependency, "reasons.NoBody", comptime.ReasonNoBody},
		{"a call through a function value", "reasons.CallsFunctionValue", comptime.ReasonDynamicCall, "", comptime.ReasonDynamicCall},
		{"a call through an interface", "reasons.CallsInterface", comptime.ReasonDynamicCall, "", comptime.ReasonDynamicCall},
		{"a function used as a value", "reasons.FunctionValue", comptime.ReasonFuncValue, "", comptime.ReasonFuncValue},
		{"a function literal", "reasons.Closure", comptime.ReasonFuncValue, "", comptime.ReasonFuncValue},
		{"a write through a parameter", "reasons.WritesParameter", comptime.ReasonForeignWrite, "", comptime.ReasonForeignWrite},
		{"a write through a pointer loaded from memory", "reasons.WritesLoaded", comptime.ReasonForeignWrite, "", comptime.ReasonForeignWrite},
		{"a channel receive", "reasons.Receives", comptime.ReasonConcurrency, "", comptime.ReasonConcurrency},
		{"a go statement", "reasons.Starts", comptime.ReasonConcurrency, "", comptime.ReasonConcurrency},
		{"a call of recover", "reasons.Recovers", comptime.ReasonRecover, "", comptime.ReasonRecover},
		{"a range over a map", "reasons.RangesOverMap", comptime.ReasonMapRange, "", comptime.ReasonMapRange},
		{"a use of package unsafe", "reasons.UsesUnsafe", comptime.ReasonUnsafe, "", comptime.ReasonUnsafe},
		{"floating-point arithmetic", "reasons.Float", comptime.ReasonFloat, "", comptime.ReasonFloat},
		{"a floating-point value that is only moved", "reasons.FloatField", comptime.ReasonFloat, "", comptime.ReasonFloat},
		{"the capacity append chose", "reasons.Appends", comptime.ReasonUnspecified, "", comptime.ReasonUnspecified},
		{"the capacity of a converted string", "reasons.StringToBytes", comptime.ReasonUnspecified, "", comptime.ReasonUnspecified},
		{"a generic function", "reasons.Generic", comptime.ReasonGeneric, "", comptime.ReasonGeneric},
		{"a call of a generic function", "reasons.CallsGeneric", comptime.ReasonGeneric, "reasons.Generic", comptime.ReasonGeneric},
		{"a type assertion, which no rule names", "reasons.Asserts", comptime.ReasonUnsupported, "", comptime.ReasonUnsupported},
		{"a defer statement, which no rule names", "reasons.Defers", comptime.ReasonUnsupported, "", comptime.ReasonUnsupported},
		{"a comparison of pointers, which no rule names", "reasons.ComparesPointers", comptime.ReasonUnsupported, "", comptime.ReasonUnsupported},
		{"a built-in that no rule names", "reasons.Prints", comptime.ReasonUnsupported, "", comptime.ReasonUnsupported},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			sym := symbol(t, report, c.symbol)
			if sym.Comptime || sym.Reason == nil {
				t.Fatalf("%s is %v, want not comptime", c.symbol, sym.Verdict)
			}
			if len(sym.Conditions) != 0 {
				t.Errorf("%s is not comptime and has conditions %v", c.symbol, sym.Conditions)
			}
			if sym.Reason.Kind != c.kind || sym.Reason.Root().Kind != c.root {
				t.Errorf("%s has the reason %s, from %s, want %s, from %s", c.symbol, sym.Reason.Kind, sym.Reason.Root().Kind, c.kind, c.root)
			}
			if cause := strings.TrimPrefix(sym.Reason.Cause, testdata); cause != c.cause {
				t.Errorf("%s names the cause %q, want %q", c.symbol, cause, c.cause)
			}
			if !strings.Contains(sym.Reason.Pos, "reasons.go:") || sym.Reason.Detail == "" {
				t.Errorf("%s has the reason %+v, want a position in reasons.go and a detail", c.symbol, *sym.Reason)
			}
		})
	}
	// The kinds cgo and unknown have tests of their own below.
	covered := []comptime.ReasonKind{comptime.ReasonCgo, comptime.ReasonUnknown}
	for _, c := range cases {
		covered = append(covered, c.kind)
	}
	for _, kind := range comptime.ReasonKinds() {
		if !slices.Contains(covered, kind) {
			t.Errorf("no test package has a symbol with the reason %s", kind)
		}
	}
	for _, pkg := range report.Packages {
		for _, sym := range pkg.Symbols {
			if sym.Comptime {
				t.Errorf("%s is comptime, and package reasons holds no such symbol", sym.Name)
			}
		}
	}
}

func TestAnalyzeRejectsAFileThatCgoGenerated(t *testing.T) {
	needCgo(t)
	report := analyze(t, comptime.Config{}, "cgo")
	for _, name := range []string{"cgo.Answer", "cgo.Plain"} {
		sym := symbol(t, report, name)
		if sym.Comptime || sym.Reason.Kind != comptime.ReasonCgo {
			t.Errorf("%s is %v, want not comptime because of cgo", name, sym.Verdict)
		}
	}
}

func TestVerdictFlowsFromADependencyToItsImporter(t *testing.T) {
	// Only the importer is named, so the verdicts of package dep reach it as facts.
	report := analyze(t, comptime.Config{}, "importer")
	if len(report.Packages) != 1 {
		t.Fatalf("the report holds %d packages, want the one that was asked for", len(report.Packages))
	}
	type testCase struct {
		description string
		symbol      string
		want        string
		cause       string
		root        comptime.ReasonKind
	}
	for _, c := range []testCase{
		{"a call of a comptime function", "importer.UsesPure", "comptime", "", ""},
		{"a call of a function that is not comptime", "importer.UsesImpure", "not comptime: dependency", "dep.Impure", comptime.ReasonVariable},
		{"a condition of a function", "importer.UsesWide", "comptime if int-size", "", ""},
		{"a condition of a constant", "importer.UsesBits", "comptime if int-size", "", ""},
		{"a condition of a type", "importer.UsesWord", "comptime if int-size", "", ""},
	} {
		t.Run(c.description, func(t *testing.T) {
			sym := symbol(t, report, c.symbol)
			if got := sym.Verdict.String(); got != c.want {
				t.Fatalf("%s is %q, want %q", c.symbol, got, c.want)
			}
			if sym.Comptime {
				return
			}
			if cause := strings.TrimPrefix(sym.Reason.Cause, testdata); cause != c.cause {
				t.Errorf("%s names the cause %q, want %q", c.symbol, cause, c.cause)
			}
			// The chain goes on in the dependency: Impure calls hidden, which reads state.
			why := sym.Reason.Why
			if why == nil || strings.TrimPrefix(why.Cause, testdata) != "dep.hidden" || sym.Reason.Root().Kind != c.root {
				t.Errorf("%s has the chain %+v, want one through dep.hidden to a %s", c.symbol, why, c.root)
			}
		})
	}
}

func TestAnalyzeSolvesRecursion(t *testing.T) {
	report := analyze(t, comptime.Config{}, "recursion")
	type testCase struct {
		description string
		symbol      string
		want        string
	}
	for _, c := range []testCase{
		{"a function that calls itself", "recursion.Factorial", "comptime"},
		{"the first of two functions that call each other", "recursion.Even", "comptime"},
		{"the second of them", "recursion.Odd", "comptime"},
		{"a function of a cycle that writes through a parameter", "recursion.Tainted", "not comptime: foreign-write"},
		{"a clean function of that cycle", "recursion.Carrier", "not comptime: dependency"},
		{"a function of a cycle that computes with int", "recursion.Sized", "comptime if int-size"},
		{"a function of that cycle that does not", "recursion.Unsized", "comptime if int-size"},
		{"a method of a type that names itself", "recursion.List.Head", "comptime"},
	} {
		t.Run(c.description, func(t *testing.T) {
			if got := symbol(t, report, c.symbol).Verdict.String(); got != c.want {
				t.Errorf("%s is %q, want %q", c.symbol, got, c.want)
			}
		})
	}
}

// paranoid is a rule set that asks for the verdict of a symbol nobody judges.
type paranoid struct{ comptime.FirstRules }

// Func looks up the receiver or the first parameter of the function, which has no verdict.
func (paranoid) Func(f *comptime.Function, look comptime.Lookup) comptime.Verdict {
	sig := f.Object.Signature()
	if sig.Recv() != nil {
		return look(sig.Recv())
	}
	if sig.Params().Len() == 0 {
		return comptime.Accepted()
	}
	return look(sig.Params().At(0))
}

func TestAnalyzeTakesOtherRulesAndRejectsWhatHasNoVerdict(t *testing.T) {
	report := analyze(t, comptime.Config{Rules: paranoid{}}, "accepted")
	type testCase struct {
		description string
		symbol      string
		want        string
	}
	for _, c := range []testCase{
		{"the new rule accepts a function without parameters", "accepted.NewCounter", "comptime"},
		{"a symbol without a verdict is not comptime", "accepted.Add8", "not comptime: unknown"},
		{"the old rule still judges constants", "accepted.Limit", "comptime"},
	} {
		t.Run(c.description, func(t *testing.T) {
			if got := symbol(t, report, c.symbol).Verdict.String(); got != c.want {
				t.Errorf("%s is %q, want %q", c.symbol, got, c.want)
			}
		})
	}
}

// confused is a rule set whose verdict on a variable is neither comptime nor has a reason.
type confused struct{ comptime.FirstRules }

// Var returns the zero verdict.
func (confused) Var(*comptime.Variable) comptime.Verdict { return comptime.Verdict{} }

func TestAnalyzeRejectsAVerdictThatContradictsItself(t *testing.T) {
	report := analyze(t, comptime.Config{Rules: confused{}}, "reasons")
	sym := symbol(t, report, "reasons.Variable")
	if sym.Comptime || sym.Reason == nil || sym.Reason.Kind != comptime.ReasonUnknown {
		t.Errorf("reasons.Variable is %v, want not comptime for want of a verdict", sym.Verdict)
	}
}

func TestReportIsSortedAndTheSameFromRunToRun(t *testing.T) {
	names := []string{"recursion", "accepted", "reasons", "importer", "dep"}
	var text, encoded [2]bytes.Buffer
	for i := range text {
		report := analyze(t, comptime.Config{}, names...)
		if err := report.WriteText(&text[i]); err != nil {
			t.Fatal(err)
		}
		if err := comptime.WriteJSON(&encoded[i], report); err != nil {
			t.Fatal(err)
		}
		if !slices.IsSortedFunc(report.Packages, func(a, b *comptime.Package) int { return strings.Compare(a.Path, b.Path) }) {
			t.Error("the packages are not sorted by path")
		}
		for _, pkg := range report.Packages {
			if !slices.IsSortedFunc(pkg.Symbols, func(a, b comptime.Symbol) int { return strings.Compare(a.Name, b.Name) }) {
				t.Errorf("the symbols of %s are not sorted by name", pkg.Path)
			}
		}
	}
	if text[0].String() != text[1].String() || encoded[0].String() != encoded[1].String() {
		t.Error("two runs over the same packages print different reports")
	}
	header, _, _ := strings.Cut(text[0].String(), "\n")
	if want := "# comptime symbols for GOOS=" + build.Default.GOOS + " GOARCH=" + build.Default.GOARCH; header != want {
		t.Errorf("the report starts with %q, want %q", header, want)
	}
	for _, want := range []string{"\n" + testdata + "accepted\n", "\tfunc Sum\tif int-size\n", "\tmethod Counter.Value\n", "\tconst Limit\n"} {
		if !strings.Contains(text[0].String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, text[0].String())
		}
	}
	for _, unwanted := range []string{"ReadsVariable", "Impure\n"} {
		if strings.Contains(text[0].String(), unwanted) {
			t.Errorf("the report lists %s, which is not comptime", unwanted)
		}
	}
}

func TestReportHoldsForTheTargetOfTheEnvironment(t *testing.T) {
	cfg := comptime.Config{Env: append(os.Environ(), "GOOS=linux", "GOARCH=386", "CGO_ENABLED=0")}
	report := analyze(t, cfg, "dep")
	if report.GOOS != "linux" || report.GOARCH != "386" {
		t.Errorf("the report holds for %s/%s, want linux/386", report.GOOS, report.GOARCH)
	}
	if got := symbol(t, report, "dep.Bits").Verdict.String(); got != "comptime if int-size" {
		t.Errorf("dep.Bits is %q, want comptime if int-size", got)
	}
}

func TestStatsCountSymbolsVerdictsAndReasons(t *testing.T) {
	st := analyze(t, comptime.Config{}, "dep", "importer").Stats()
	type testCase struct {
		description string
		got, want   int
	}
	for _, c := range []testCase{
		{"packages", st.Packages, 2},
		{"packages that were not analyzed", st.Failed, 0},
		{"symbols", st.TotalSymbols, 9},
		{"comptime symbols", st.TotalComptime, 7},
		{"constants", st.Symbols[comptime.KindConst], 2},
		{"comptime constants", st.Comptime[comptime.KindConst], 2},
		{"functions", st.Symbols[comptime.KindFunc], 7},
		{"comptime functions", st.Comptime[comptime.KindFunc], 5},
		{"symbols that are comptime without conditions", st.Conditions["none"], 2},
		{"symbols that are comptime for one size of int", st.Conditions["int-size"], 5},
		{"symbols with the reason dependency", st.Reasons["dependency"], 2},
		{"chains that end at a package-level variable", st.RootReasons["package-variable: reference to a package-level variable"], 2},
	} {
		t.Run(c.description, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("the statistics count %d, want %d", c.got, c.want)
			}
		})
	}
	var text bytes.Buffer
	if err := st.WriteText(&text); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GOOS=" + st.GOOS + " GOARCH=" + st.GOARCH, "symbols   9, of which 7 comptime", "       2  dependency\n"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("the statistics lack %q:\n%s", want, text.String())
		}
	}
}

func TestAnalyzeReportsAPackageItCannotLoad(t *testing.T) {
	report, err := comptime.Analyze(comptime.Config{}, testdata+"missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Packages) != 1 || report.Packages[0].Error == "" || report.Stats().Failed != 1 {
		t.Errorf("the report is %+v, want one package with an error", report.Packages)
	}
}
