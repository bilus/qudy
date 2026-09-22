package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lispg is the transpiler, built once for every test.
var lispg transpiler

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lispg")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "lispg")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", binary, ".").CombinedOutput(); err != nil {
		panic("build the transpiler: " + err.Error() + "\n" + string(out))
	}
	lispg = newTranspiler(binary)
	m.Run()
}

// transpiler runs the lispg binary.
type transpiler struct {
	binary string
}

// newTranspiler returns the transpiler at a binary.
func newTranspiler(binary string) transpiler {
	return transpiler{binary: binary}
}

// compileFile transpiles one of the example files.
func (l transpiler) compileFile(t *testing.T, name string) *program {
	t.Helper()
	generated, err := exec.CommandContext(t.Context(), l.binary, filepath.Join("examples", name)).Output()
	if err != nil {
		t.Fatalf("transpile %s: %v", name, err)
	}
	return newProgram(t, generated)
}

// compile transpiles a source, given on standard input.
func (l transpiler) compile(t *testing.T, source string) *program {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), l.binary)
	cmd.Stdin = strings.NewReader(source)
	generated, err := cmd.Output()
	if err != nil {
		t.Fatalf("transpile %q: %v", source, err)
	}
	return newProgram(t, generated)
}

// reject transpiles a source that lispg must refuse, and returns its message.
func (l transpiler) reject(t *testing.T, source string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), l.binary)
	cmd.Stdin = strings.NewReader(source)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("lispg accepted %q:\n%s", source, out)
	}
	return string(out)
}

// program is a transpiled Go program in a directory of its own.
type program struct {
	source string
	dir    string
	binary string
}

// newProgram writes a transpiled program to a temporary directory.
func newProgram(t *testing.T, generated []byte) *program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), generated, 0o600); err != nil {
		t.Fatal(err)
	}
	return &program{source: string(generated), dir: dir}
}

// build compiles the program and returns the compiler's output.
func (p *program) build() ([]byte, error) {
	binary := filepath.Join(p.dir, "program")
	out, err := exec.Command("go", "build", "-o", binary, filepath.Join(p.dir, "main.go")).CombinedOutput()
	if err == nil {
		p.binary = binary
	}
	return out, err
}

// buildError returns the diagnostics of a program that Go must reject.
func (p *program) buildError(t *testing.T) string {
	t.Helper()
	out, err := p.build()
	if err == nil {
		t.Fatalf("go build accepted the program:\n%s", p.source)
	}
	return string(out)
}

// exec builds the program once and runs it in a directory, with arguments.
func (p *program) exec(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	if p.binary == "" {
		if out, err := p.build(); err != nil {
			t.Fatalf("go build: %v\n%s\n%s", err, out, p.source)
		}
	}
	cmd := exec.CommandContext(t.Context(), p.binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// run runs the program in the test's directory and returns its output.
func (p *program) run(t *testing.T, args ...string) string {
	t.Helper()
	return p.runIn(t, "", args...)
}

// runIn runs the program in a directory and returns its output.
func (p *program) runIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := p.exec(t, dir, args...)
	if err != nil {
		t.Fatalf("the program failed: %v\n%s\n%s", err, out, p.source)
	}
	return out
}

// fail runs the program, which must exit with an error, and returns its output.
func (p *program) fail(t *testing.T, args ...string) string {
	t.Helper()
	out, err := p.exec(t, "", args...)
	if err == nil {
		t.Fatalf("the program succeeded with %q:\n%s", args, out)
	}
	return out
}

func TestExamples(t *testing.T) {
	type testCase struct {
		file string
		want string
	}
	for _, c := range []testCase{
		{"hello.lisp", "Hello JOHN (10)\n"},
		{"features.lisp", "15\n120 Hello John 2.5\nyes\n16\nwhen\nelse\n"},
		{"expressions.lisp", "body 40\n0\n\"yes\" \"\"\n1.5 0\ntrue false\nlet 42\n8 12\n0 0 0 0\ncondition\n5\n"},
		{"lists.lisp", "map: [1 4 9 16]\nreduce: 10\nfilter: [2 4]\noriginal: [1 2 3 4]\nempty: [] 10 []\nclosure: [11 12 13]\n"},
		{"list_types.lisp", "1 [2] true\n[]int64 []float64 []string []bool\n[hi hello world] [1.5 2.5]\n[] []\ntrue\n[] 5\n[[1 2] []]\n[7 8]\n[1 2 3] [2 3] [9 2 3] [8 2 3]\n[1]\n"},
	} {
		t.Run(c.file, func(t *testing.T) {
			program := lispg.compileFile(t, c.file)
			if got := program.run(t); got != c.want {
				t.Errorf("the program wrote\n%q\nwant\n%q\n%s", got, c.want, program.source)
			}
		})
	}
}

// fixture builds a directory tree of 15 bytes and returns its root.
func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fixture")
	for _, name := range []string{"sub/deeper", "zempty"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{"a.txt": "abc", "sub/b.bin": "12345", "sub/deeper/c.bin": "1234567"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDirectoryTree(t *testing.T) {
	tree := lispg.compileFile(t, "tree.lisp")
	root := fixture(t)
	linkLine := ""
	if err := os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "link")); err == nil {
		linkLine = "  link (not counted)\n"
	}
	listing := " (15 B)\n  a.txt (3 B)\n" + linkLine + "  sub/ (12 B)\n    b.bin (5 B)\n    deeper/ (7 B)\n      c.bin (7 B)\n  zempty/ (0 B)\n"

	if got, want := tree.run(t, root+string(os.PathSeparator)), "fixture/"+listing; got != want {
		t.Errorf("a root argument gave\n%s\nwant\n%s", got, want)
	}
	if got, want := tree.runIn(t, root), "./"+listing; got != want {
		t.Errorf("the default root gave\n%s\nwant\n%s", got, want)
	}
	for _, name := range []string{"missing", "a.txt"} {
		t.Run(name, func(t *testing.T) {
			out := tree.fail(t, filepath.Join(root, name))
			if !strings.Contains(out, name) || strings.Contains(out, "panic:") {
				t.Errorf("want a reported walk error for %s, got\n%s", name, out)
			}
		})
	}
}

func TestSemantics(t *testing.T) {
	source := `(package main) (import "fmt")
(defn choose ^int64 [^bool b] (let [x 40] (if b (+ x 2) 0)))
(defn main []
  (let [x 5 x (+ x 1) unused 99] (fmt.Println x))
  (let [x 7] (let [x (+ x 1)] (fmt.Println x)) (fmt.Println x))
  (fmt.Println ((fn ^int64 [^int64 x] (* x x)) 4))
  (fmt.Println (choose true) (choose false))
  (let [^int64 value (if true 9 0)] (fmt.Println value))
  (fmt.Printf "%T %T %T\n" 1 -2 1.0)
  (fmt.Println "a\n\"b\"\\c")
  (when false (fmt.Println "bad"))
  (if true (fmt.Println "one branch"))
  (+ 1 2)
  (let [] (fmt.Println "empty bindings"))
  (fmt.Println (or false true) (not= 1 2) (% 7 3)))`
	want := "6\n8\n7\n16\n42 0\n9\nint64 int64 float64\na\n\"b\"\\c\none branch\nempty bindings\ntrue true 1\n"
	program := lispg.compile(t, source)
	if got := program.run(t); got != want {
		t.Errorf("the program wrote\n%q\nwant\n%q\n%s", got, want, program.source)
	}
}

func TestRejectedSources(t *testing.T) {
	for _, source := range []string{
		"", "(package main", "(package main]", ")", "(package main) (package foo)",
		"(defn main [] 1)", "(package main) (defn main [] (let [x] 1))",
		"(package main) (defn main [] (if true))",
		"(package main) (defn f [x] x)",
		"(package main) (defn f ^bogus [] 1)",
		"(package main) (defn main [] (let [^bogus x 1] x))",
		"(package main) (defn main [] (fmt.Println (if true 1 2)))",
		"(package main) (defn main [] (when))",
		"(package main) (defn main [] (package inner))",
		"(package main) (defn main [] (import \"fmt\"))",
		"(package main) (defn main [] (defn inner [] 1))",
		"(package main) (let [x 1] x)",
		"(package main) (defn main [] (let [x 9223372036854775808] x))",
		"(package main) (defn f [^int64 x ^int64 x] x)",
		"(package main) (defn main [] 1) (import \"fmt\")",
		"(package main) (defn main [] \"unterminated)",
		"(package main) (defn main [] [])",
		"(package main) (defn f ^[bogus] [] [])",
		"(package main) (defn f ^[] [] [])",
		"(package main) (defn f ^[int64 string] [] [])",
		"(package main) (defn main [^(fn int64 int64) f] 1)",
		"(package main) (defn main [] (first))",
		"(package main) (defn main [] (rest [1] [2]))",
		"(package main) (defn main [] (cons 1))",
		"(package main) (defn _lispgList [] 1)",
		"(package main) (defn ^int64 f [] 1)",
		"(package main) (defn f ^fs.bad.name [] nil)",
	} {
		t.Run(source, func(t *testing.T) {
			if out := lispg.reject(t, source); !strings.Contains(out, "lispg:") {
				t.Errorf("the message lacks the lispg: prefix:\n%s", out)
			}
		})
	}
}

// TestTypeErrors leaves heterogeneous lists and mismatched functions to the Go compiler.
func TestTypeErrors(t *testing.T) {
	type testCase struct {
		body       string
		diagnostic string
	}
	for _, c := range []testCase{
		{`(let [xs [1 "x"]] xs)`, "string"},
		{`(let [xs [1 2.5]] xs)`, "float64"},
		{`(let [^[int64] xs ["x"]] xs)`, "string"},
		{`(cons "x" [1 2])`, "string"},
		{`(first 1)`, "[]"},
		{`((fn ^int64 [^(fn [int64] int64) f] (f 1)) (fn ^string [^int64 x] "x"))`, "func"},
	} {
		t.Run(c.body, func(t *testing.T) {
			program := lispg.compile(t, "(package main) (defn main [] "+c.body+")")
			if out := program.buildError(t); !strings.Contains(out, c.diagnostic) {
				t.Errorf("want a diagnostic naming %q, got\n%s", c.diagnostic, out)
			}
		})
	}
}

func TestLineLimit(t *testing.T) {
	for _, name := range []string{"compiler.go", "compiler_gen.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(data), "\n"); n > 500 {
			t.Errorf("%s: %d lines exceeds 500", name, n)
		}
	}
}
