package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

var (
	fset     = token.NewFileSet()
	imports  = importer.ForCompiler(fset, "source", nil)
	programs int
)

// check type-checks a program and reports the first error.
func check(src string) error {
	file, err := parser.ParseFile(fset, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	_, err = (&types.Config{Importer: imports}).Check("main", fset, []*ast.File{file}, nil)
	return err
}

// Every generated program, pure or planted, is well-typed Go that gofmt accepts.
func TestGeneratedProgramsTypeCheck(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := Generate(t, rapid.Bool().Draw(t, "plant"))
		programs++
		if err := check(p.Src); err != nil {
			t.Fatalf("%v\n%s", err, p.Src)
		}
		if _, err := format.Source([]byte(p.Src)); err != nil {
			t.Fatalf("%v\n%s", err, p.Src)
		}
	})
	t.Logf("%d programs checked", programs)
}

// build compiles a program for one target and returns the binary.
func build(t *testing.T, dir, name, src string, env ...string) string {
	t.Helper()
	file := filepath.Join(dir, name+".go")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, name+"."+strings.NewReplacer("=", "", " ", "").Replace(strings.Join(env, "")))
	cmd := exec.Command("go", "build", "-o", bin, file)
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s %v: %v\n%s\n%s", name, env, err, out, src)
	}
	return bin
}

// run executes a binary and returns what it printed, or the error text.
func run(bin string, env ...string) string {
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\n[%v]", out, err)
	}
	return string(out)
}

// observe runs every dynamic oracle on a program and reports which ones saw a difference.
func observe(t *testing.T, dir, name, src string) map[string]bool {
	amd64 := build(t, dir, name, src, "GOARCH=amd64", "GOAMD64=v1")
	v3 := build(t, dir, name, src, "GOARCH=amd64", "GOAMD64=v3")
	x86 := build(t, dir, name, src, "GOARCH=386")
	first := run(amd64)
	rounds := strings.SplitN(first, "--\n", 2)
	seen := map[string]bool{"repeat": len(rounds) != 2 || rounds[0] != rounds[1]}
	for range 5 {
		seen["rerun"] = seen["rerun"] || run(amd64) != first
	}
	seen["env"] = run(amd64, "HOME=/nonexistent/elsewhere", "TZ=Asia/Tokyo") != first
	seen["v3"] = run(v3) != first
	seen["386"] = run(x86) != first
	return seen
}

var oracles = []string{"repeat", "rerun", "env", "v3", "386"}

// row formats one line of the table.
func row(name string, seen map[string]bool) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%-36s", name)
	for _, o := range oracles {
		mark := "."
		if seen[o] {
			mark = "X"
		}
		fmt.Fprintf(&b, " %-6s", mark)
	}
	return b.String()
}

// Which dynamic oracle sees which impurity? One program per catalog entry, plus pure controls.
func TestOracleTable(t *testing.T) {
	if testing.Short() {
		t.Skip("builds about a hundred binaries")
	}
	dir := t.TempDir()
	var table []string
	table = append(table, row("", map[string]bool{})[:36]+" "+strings.Join(oracles, "  "))
	for i, imp := range impurities {
		src := "package main\n\nimport (\n\t\"fmt\"\n"
		for _, path := range imp.imports {
			src += fmt.Sprintf("\t%q\n", path)
		}
		src += ")\n\n" + imp.decls + "func impure() int64 {\n\t" + imp.body + "\n}\n\nfunc round() {\n\tfmt.Println(impure())\n}\n\n" +
			"func main() {\n\tround()\n\tfmt.Println(\"--\")\n\t" + imp.between + "\n\tround()\n}\n"
		if err := check(src); err != nil {
			t.Fatalf("%s: %v", imp.name, err)
		}
		table = append(table, row(imp.name, observe(t, dir, fmt.Sprintf("imp%d", i), src)))
	}
	pure := rapid.Custom(func(t *rapid.T) Program { return Generate(t, false) })
	differing := 0
	for seed := range 25 {
		p := pure.Example(seed)
		for o, saw := range observe(t, dir, fmt.Sprintf("pure%d", seed), p.Src) {
			if saw {
				differing++
				t.Errorf("pure program %d differs under %s:\n%s", seed, o, p.Src)
			}
		}
	}
	table = append(table, fmt.Sprintf("%-36s %d of 25 pure programs differ under any oracle", "pure controls", differing))
	t.Log("\n" + strings.Join(table, "\n"))
}

// lines returns what the first round printed for each function.
func lines(out string) map[string]string {
	byFunc := map[string]string{}
	for _, line := range strings.Split(strings.SplitN(out, "--\n", 2)[0], "\n") {
		if name, value, ok := strings.Cut(line, " "); ok {
			byFunc[name] = value
		}
	}
	return byFunc
}

// Is the generator's own claim about each function true? Plant a loud impurity and compare two runs.
func TestGroundTruth(t *testing.T) {
	if testing.Short() {
		t.Skip("builds sixty binaries")
	}
	dir := t.TempDir()
	planted := rapid.Custom(func(t *rapid.T) Program { return Generate(t, true) })
	type tally struct{ differ, total int }
	var clean, victim, callers tally
	byLayer := map[string]*tally{}
	for seed, built := 0, 0; built < 60; seed++ {
		p := planted.Example(seed)
		if p.Impurity != "clock" && p.Impurity != "random number" {
			continue
		}
		built++
		bin := build(t, dir, fmt.Sprintf("truth%d", seed), p.Src, "GOARCH=amd64")
		first, second := lines(run(bin)), lines(run(bin))
		tainted := map[string]bool{}
		for _, name := range p.Tainted {
			tainted[name] = true
		}
		for _, name := range p.Funcs {
			differs := first[name] != second[name]
			count := &clean
			switch {
			case name == p.Victim:
				count = &victim
				for _, l := range p.Layers {
					if byLayer[l] == nil {
						byLayer[l] = &tally{}
					}
					byLayer[l].total++
					if differs {
						byLayer[l].differ++
					}
				}
			case tainted[name]:
				count = &callers
			case differs:
				t.Errorf("seed %d: %s is outside the tainted set, and its result changed between runs:\n%s", seed, name, p.Src)
			}
			count.total++
			if differs {
				count.differ++
			}
		}
	}
	t.Logf("outside the tainted set: %d of %d changed between two runs", clean.differ, clean.total)
	t.Logf("victims:                 %d of %d changed", victim.differ, victim.total)
	t.Logf("callers of a victim:     %d of %d changed", callers.differ, callers.total)
	for _, l := range layers {
		if c := byLayer[l.name]; c != nil {
			t.Logf("  victim behind %-36q %d of %d changed", l.name, c.differ, c.total)
		}
	}
}

// Print a sample planted program, to read.
func TestSample(t *testing.T) {
	planted := rapid.Custom(func(t *rapid.T) Program { return Generate(t, true) })
	for seed := range 400 {
		p := planted.Example(seed)
		if len(p.Layers) == 2 && len(p.Funcs) == 2 && len(p.Src) < 1400 {
			src, _ := format.Source([]byte(p.Src))
			t.Logf("seed %d: impurity %q behind %v; victim %s; tainted %v\n%s", seed, p.Impurity, p.Layers, p.Victim, p.Tainted, src)
			return
		}
	}
}
