package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrograms(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "lispg")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build transpiler: %v\n%s", err, out)
	}
	for _, tc := range []struct{ file, want string }{
		{"hello.lisp", "Hello JOHN (10)\n"},
		{"features.lisp", "15\n120 Hello John 2.5\nyes\n16\nwhen\nelse\n"},
		{"expressions.lisp", "body 40\n0\n\"yes\" \"\"\n1.5 0\ntrue false\nlet 42\n8 12\n0 0 0 0\ncondition\n5\n"},
		{"lists.lisp", "map: [1 4 9 16]\nreduce: 10\nfilter: [2 4]\noriginal: [1 2 3 4]\nempty: [] 10 []\nclosure: [11 12 13]\n"},
		{"list_types.lisp", "1 [2] true\n[]int64 []float64 []string []bool\n[hi hello world] [1.5 2.5]\n[] []\ntrue\n[] 5\n[[1 2] []]\n[7 8]\n[1 2 3] [2 3] [9 2 3] [8 2 3]\n[1]\n"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			cmd := exec.Command(binary, filepath.Join("examples", tc.file))
			generated, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "main.go")
			if err := os.WriteFile(path, generated, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("go", "run", path).CombinedOutput()
			if err != nil || string(out) != tc.want {
				t.Fatalf("generated program: %v\nwant %q\ngot %q\n%s", err, tc.want, out, generated)
			}
		})
	}

	t.Run("directory tree", func(t *testing.T) {
		generated, err := exec.Command(binary, "examples/tree.lisp").Output()
		if err != nil {
			t.Fatalf("transpile tree: %v", err)
		}
		dir := t.TempDir()
		path, program := filepath.Join(dir, "main.go"), filepath.Join(dir, "tree")
		if err := os.WriteFile(path, generated, 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("go", "build", "-o", program, path).CombinedOutput(); err != nil {
			t.Fatalf("build tree: %v\n%s\n%s", err, out, generated)
		}
		root := filepath.Join(dir, "fixture")
		for _, name := range []string{"sub/deeper", "zempty"} {
			if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
				t.Fatal(err)
			}
		}
		for name, data := range map[string]string{"a.txt": "abc", "sub/b.bin": "12345", "sub/deeper/c.bin": "1234567"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
		linkLine := ""
		if err := os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "link")); err == nil {
			linkLine = "  link (not counted)\n"
		}
		tail := " (15 B)\n  a.txt (3 B)\n" + linkLine + "  sub/ (12 B)\n    b.bin (5 B)\n    deeper/ (7 B)\n      c.bin (7 B)\n  zempty/ (0 B)\n"
		cmd := exec.Command(program, root+string(os.PathSeparator))
		if out, err := cmd.CombinedOutput(); err != nil || string(out) != "fixture/"+tail {
			t.Fatalf("tree: %v\n%s", err, out)
		}
		cmd = exec.Command(program)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil || string(out) != "./"+tail {
			t.Fatalf("default root: %v\n%s", err, out)
		}
		for _, name := range []string{"missing", "a.txt"} {
			out, err := exec.Command(program, filepath.Join(root, name)).CombinedOutput()
			if err == nil || !strings.Contains(string(out), name) || strings.Contains(string(out), "panic:") {
				t.Fatalf("expected a reported walk error for %s: %v\n%s", name, err, out)
			}
		}
	})

	t.Run("semantics", func(t *testing.T) {
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
		cmd := exec.Command(binary)
		cmd.Stdin = strings.NewReader(source)
		generated, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "main.go")
		if err := os.WriteFile(path, generated, 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("go", "run", path).CombinedOutput()
		want := "6\n8\n7\n16\n42 0\n9\nint64 int64 float64\na\n\"b\"\\c\none branch\nempty bindings\ntrue true 1\n"
		if err != nil || string(out) != want {
			t.Fatalf("%v\nwant %q\ngot %q\n%s", err, want, out, generated)
		}
	})

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
		t.Run("reject:"+source, func(t *testing.T) {
			cmd := exec.Command(binary)
			cmd.Stdin = strings.NewReader(source)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "lispg:") {
				t.Fatalf("wanted error, got %v: %s", err, out)
			}
		})
	}
	// Go must reject heterogeneous lists and incompatible higher-order arguments.
	for _, tc := range []struct{ body, diagnostic string }{
		{`(let [xs [1 "x"]] xs)`, "string"},
		{`(let [xs [1 2.5]] xs)`, "float64"},
		{`(let [^[int64] xs ["x"]] xs)`, "string"},
		{`(cons "x" [1 2])`, "string"},
		{`(first 1)`, "[]"},
		{`((fn ^int64 [^(fn [int64] int64) f] (f 1)) (fn ^string [^int64 x] "x"))`, "func"},
	} {
		t.Run("type error:"+tc.body, func(t *testing.T) {
			cmd := exec.Command(binary)
			cmd.Stdin = strings.NewReader("(package main) (defn main [] " + tc.body + ")")
			generated, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "main.go")
			if err := os.WriteFile(path, generated, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("go", "build", "-o", filepath.Join(dir, "program"), path).CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.diagnostic) {
				t.Fatalf("wanted %q type error, got %v: %s", tc.diagnostic, err, out)
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
