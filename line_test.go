package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// mapped is a template with a loop, a four-line run and a gensym.
var mapped = strings.Join([]string{
	"//go:build qudy",
	"",
	"package p",
	"",
	"func f() {",
	"	for i := range xs {",
	"		//`x := ~i",
	"	}",
	"	//`a ~p",
	"	//`b",
	"	//`c ~q ~r",
	"	//`d\\",
	"	y := 1",
	"	_ = y",
	"	//`e err#",
	"}",
	"",
}, "\n")

func TestCompileWritesLineDirectives(t *testing.T) {
	gen, err := qudy.Compile("t.go", []byte(mapped), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		description string
		want        string
	}
	for _, c := range []testCase{
		{"the package clause, after the constraint", "//line t.go:3\npackage p"},
		{"the line after the symbol generator", "//line t.go:6\n\tfor i := range xs {"},
		{"a single output line, so that its arguments map to it", "//line t.go:6\n\t\tg.generate(`x := %v\n`, i)"},
		{"the line after a run", "//line t.go:8\n\t}"},
		{"a run of several lines, by its first line", "//line t.go:9\n\tg.generate(`a %v\nb\nc %v %v\nd`, /*line t.go:9*/ p /*line t.go:11*/, q, r)"},
		{"the line after a gensym declaration", "err_qd := qudyGensym(\"err\")\n//line t.go:14\n\tg.generate(`e %v\n`, err_qd)"},
		{"the closing brace", "//line t.go:16\n}"},
	} {
		t.Run(c.description, func(t *testing.T) {
			if !strings.Contains(string(gen), c.want) {
				t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, c.want)
			}
		})
	}
	if lines := strings.Count(string(gen), "//line "); lines != 8 {
		t.Errorf("the generator holds %d line directives, want 8:\n%s", lines, gen)
	}
}

// broken is a template with a scaffolding error and three bad interpolations.
var broken = strings.Join([]string{
	"package main",
	"",
	"func main() {",
	"	x := 1",
	"	//`~x ~undefinedA",
	"	var s string = 2",
	"	_ = s",
	"	//`a",
	"	//`b ~undefinedB",
	"	//`c",
	"	for range 2 {",
	"		//`~undefinedC",
	"	}",
	"}",
	"",
}, "\n")

func TestTheCompilerReportsErrorsAtTemplateLines(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	gen, err := qudy.Compile("t.go", []byte(broken), "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{"go.mod": "module gen\n\ngo 1.25\n", "t_gen.go": string(gen)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	build := exec.CommandContext(t.Context(), goTool, "build", "-gcflags=-e", "-o", os.DevNull, ".")
	build.Dir = dir
	out, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("the generator built:\n%s", gen)
	}
	for _, want := range []string{
		"t.go:5: undefined: undefinedA",
		"t.go:6: cannot use 2 (untyped int constant) as string value",
		"t.go:9: undefined: undefinedB",
		"t.go:12: undefined: undefinedC",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the compiler reported\n%s\nwant it to hold\n%s", out, want)
		}
	}
	if strings.Contains(string(out), "t_gen.go:") {
		t.Errorf("the compiler named the generator:\n%s", out)
	}
}

func TestCompileResyncsAfterADroppedDirective(t *testing.T) {
	src := "package p\n\n//go:generate qudy -o p_gen.go p.go\n\nfunc f() {\n\t//`x\n}\n"
	gen, err := qudy.Compile("t.go", []byte(src), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	if want := "//line t.go:5\nfunc f() {"; !strings.Contains(string(gen), want) {
		t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
	}
}
