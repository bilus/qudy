package comptime_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/comptime"
)

// neverComptime lists symbols of the standard library whose result depends on when or where they run.
var neverComptime = []string{
	"time.Now", "time.Since", "os.Getenv", "os.ReadFile", "os.Getpid", "math/rand.Int", "math/rand/v2.Int",
	"crypto/rand.Read", "runtime.GC", "runtime.NumGoroutine", "sync/atomic.AddInt64", "fmt.Println",
}

// watched lists symbols whose verdict shows how far the rules reach.
var watched = []string{
	"unicode/utf8.RuneLen", "unicode/utf8.ValidRune", "math/bits.OnesCount64", "math/bits.Reverse8",
	"strings.Repeat", "strconv.Itoa", "path.Base",
}

// stdSymbol returns a symbol of the standard library, named as path.Name.
func stdSymbol(t *testing.T, report *comptime.Report, name string) comptime.Symbol {
	t.Helper()
	dot := strings.LastIndex(name, ".")
	sym, ok := lookup(report, name[:dot], name[dot+1:])
	if !ok {
		t.Fatalf("the report has no symbol %s", name)
	}
	return sym
}

// chain renders a verdict and, for a rejection, every reason down to the construct that started it.
func chain(v comptime.Verdict) string {
	text := v.String()
	for r := v.Reason; r != nil; r = r.Why {
		text += "\n\t" + string(r.Kind) + ": " + r.Detail
		if r.Cause != "" {
			text += " " + r.Cause
		}
		text += " at " + r.Pos
	}
	return text
}

func TestStandardLibrary(t *testing.T) {
	if testing.Short() {
		t.Skip("the standard library takes seconds and more than a gigabyte to analyze")
	}
	report, err := comptime.Analyze(comptime.Config{}, "std")
	if err != nil {
		t.Fatal(err)
	}
	st := report.Stats()
	t.Logf("%s/%s: %d packages, %d exported symbols, %d comptime", st.GOOS, st.GOARCH, st.Packages, st.TotalSymbols, st.TotalComptime)
	if st.Packages < 100 || st.Failed != 0 {
		t.Errorf("%d packages were analyzed and %d were not, want the whole standard library", st.Packages, st.Failed)
	}
	t.Run("a symbol that depends on its surroundings is never comptime", func(t *testing.T) {
		for _, name := range neverComptime {
			sym := stdSymbol(t, report, name)
			if sym.Comptime {
				t.Errorf("%s is listed as %v", name, sym.Verdict)
			}
			t.Logf("%s: %s", name, chain(sym.Verdict))
		}
	})
	t.Run("every symbol a function depends on has a verdict", func(t *testing.T) {
		if n := st.Reasons[string(comptime.ReasonUnknown)]; n != 0 {
			t.Errorf("%d symbols were rejected for want of a verdict", n)
		}
		for root, n := range st.RootReasons {
			if strings.HasPrefix(root, string(comptime.ReasonUnknown)+":") {
				t.Errorf("%d symbols depend on a symbol without a verdict: %s", n, root)
			}
		}
	})
	t.Run("a variable is never comptime", func(t *testing.T) {
		if n := st.Comptime[comptime.KindVar]; n != 0 || st.Symbols[comptime.KindVar] == 0 {
			t.Errorf("%d of %d variables are comptime, want none of many", n, st.Symbols[comptime.KindVar])
		}
	})
	for _, name := range watched {
		t.Logf("%s: %s", name, chain(stdSymbol(t, report, name).Verdict))
	}
}
