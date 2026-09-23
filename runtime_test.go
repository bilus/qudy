package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// runWithRuntime compiles the templates with a runtime file into one package and runs it.
func runWithRuntime(t *testing.T, emit string, templates, sources map[string]string) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	files := map[string]string{"go.mod": "module gen\n\ngo 1.25\n"}
	for name, text := range sources {
		files[name] = text
	}
	for name, src := range templates {
		gen, runtime, err := qudy.CompileWithRuntime(name, []byte(src), emit)
		if err != nil {
			t.Fatal(err)
		}
		files[strings.TrimSuffix(name, ".go")+"_gen.go"] = string(gen)
		files["qudy_runtime.go"] = string(runtime)
	}
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := exec.CommandContext(t.Context(), goTool, "run", ".")
	run.Dir = dir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the generator fails: %v\n%s", err, out)
	}
	return string(out)
}

// captures captures a called function, nests a capture, and writes normally around both.
const captures = `package main

func main() {
	//` + "`" + `before
	outer := qudyCapture(func() {
		segment("a")
		inner := qudyCapture(func() {
			segment("b")
		})
		//` + "`" + `[~inner]\
	})
	//` + "`" + `~outer
	//` + "`" + `after
}

func segment(s string) {
	//` + "`" + `~s/\
}
`

// pushes redirects the output of a called function until the deferred pop.
const pushes = `package main

import (
	"fmt"
	"strings"
)

func main() {
	p := path([]string{"usr", "bin"})
	//` + "`" + `const dir = "~p"
}

func path(segments []string) string {
	var b strings.Builder
	defer qudyPush(func(format string, args ...any) { fmt.Fprintf(&b, format, args...) })()
	for _, s := range segments {
		segment(s)
	}
	return b.String()
}

func segment(s string) {
	//` + "`" + `~s/\
}
`

// gensyms asks for names in two functions, a direct call and two scopes.
const gensyms = `package main

func main() {
	first()
	second()
	third := qudyGensym("err")
	//` + "`" + `~third
	scoped()
	scoped()
	//` + "`" + `x#
}

func first() {
	//` + "`" + `err#
}

func second() {
	//` + "`" + `err#
}

func scoped() {
	end := qudyScope()
	defer end()
	//` + "`" + `a# b#
	inner()
}

func inner() {
	defer qudyScope()()
	//` + "`" + `c#
}
`

func TestTheRuntimeFileExtendsAGenerator(t *testing.T) {
	type testCase struct {
		description string
		template    string
		want        string
	}
	for _, c := range []testCase{
		{"qudyCapture returns the output of called functions, and nests", captures, "before\na/[b/]\nafter\n"},
		{"qudyPush redirects called functions until its pop", pushes, "const dir = \"usr/bin/\"\n"},
		{"one counter serves every function, and a scope restores it", gensyms, "err_qd1\nerr_qd2\nerr_qd3\na_qd4 b_qd5\nc_qd6\na_qd4 b_qd5\nc_qd6\nx_qd4\n"},
	} {
		t.Run(c.description, func(t *testing.T) {
			got := runWithRuntime(t, "fmt.Printf", map[string]string{"main.go": c.template}, nil)
			if got != c.want {
				t.Errorf("the generator wrote %q, want %q", got, c.want)
			}
		})
	}
}

// receiver is a type with a method as its emit function, as livegen has.
const receiver = `package main

import (
	"bytes"
	"os"
)

type generator struct{ buf bytes.Buffer }

func (g *generator) generate(format string, args ...any) {
	g.buf.WriteString(sprintf(format, args...))
}

func main() {
	g := &generator{}
	g.first()
	os.Stdout.Write(g.buf.Bytes())
}
`

// formats is the only file of the test package that imports fmt.
const formats = `package main

import "fmt"

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }
`

func TestTwoTemplatesShareTheRuntimeFileThroughAMethodEmitter(t *testing.T) {
	templates := map[string]string{
		"first.go":  "package main\n\nfunc (g *generator) first() {\n\ts := qudyCapture(func() {\n\t\tg.second()\n\t})\n\t//`first [~s]\n}\n",
		"second.go": "package main\n\nfunc (g *generator) second() {\n\t//`second\\\n}\n",
	}
	sources := map[string]string{"generator.go": receiver, "formats.go": formats}
	if got, want := runWithRuntime(t, "g.generate", templates, sources), "first [second]\n"; got != want {
		t.Errorf("the generators wrote %q, want %q", got, want)
	}
}

func TestCompileWithRuntimeLeavesTheSymbolGeneratorToTheRuntimeFile(t *testing.T) {
	gen, runtime, err := qudy.CompileWithRuntime("test.go", []byte(template("//`a := errName#")), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"qudyOut := qudyDispatch(func(format string, args ...any) { g.generate(format, args...) })",
		`errName_qd := qudyGensym("errName")`,
		"qudyOut(`a := %v\n`, errName_qd)",
	} {
		if !strings.Contains(string(gen), want) {
			t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
		}
	}
	if strings.Contains(string(gen), builtin) {
		t.Errorf("the generator declares a symbol generator of its own:\n%s", gen)
	}
	for _, want := range []string{"package p\n", "func qudyCapture(f func()) string {", "func qudyGensym(want string) string {", "func qudyScope() func() {"} {
		if !strings.Contains(string(runtime), want) {
			t.Errorf("the runtime file is\n%s\nwant it to hold\n%s", runtime, want)
		}
	}
}

func TestCompileWithRuntimeAcceptsAQudyGensymCallOutsideAFunctionBody(t *testing.T) {
	gen, _, err := qudy.CompileWithRuntime("test.go", []byte("package p\n\nvar x = qudyGensym(\"x\")\n"), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	if want := `var x = qudyGensym("x")`; !strings.Contains(string(gen), want) {
		t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
	}
}

func TestCompileWithoutRuntimeIsUnchanged(t *testing.T) {
	gen := compile(t, template("//`a := errName#"))
	if strings.Contains(gen, "qudyOut") || strings.Contains(gen, "qudyDispatch") {
		t.Errorf("a plain compile mentions the runtime file:\n%s", gen)
	}
}
