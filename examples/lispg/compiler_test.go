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

	t.Run("semantics", func(t *testing.T) {
		source := `(package main) (import "fmt")
(defn ^int64 choose [^bool b] (let [x 40] (if b (+ x 2) 0)))
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
		"(package main) (defn ^bogus f [] 1)",
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
