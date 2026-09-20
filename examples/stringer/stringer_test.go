package main_test

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

func TestGeneratorIsUpToDate(t *testing.T) {
	src, err := os.ReadFile("stringer.go")
	if err != nil {
		t.Fatal(err)
	}
	want, err := qudy.Compile("stringer.go", src, "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	checkedIn, err := os.ReadFile("stringer_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	_, got, found := strings.Cut(string(checkedIn), "DO NOT EDIT.\n\n")
	if !found || got != string(want) {
		t.Errorf("stringer_gen.go is stale, run go generate:\n%s\nwant\n%s", got, want)
	}
}

// enum declares constants with the names of the generated receiver and local.
const enum = `package axis

type axis int

const (
	x axis = iota
	y
	z
	v
)
`

// enumTest checks the generated String method.
const enumTest = `package axis

import "testing"

func TestString(t *testing.T) {
	for value, want := range map[axis]string{x: "x", v: "v", axis(9): "axis(9)"} {
		if got := value.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
`

func TestGeneratedCodeBuildsWithConstantsNamedXAndV(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	run := exec.CommandContext(t.Context(), goTool, "run", ".", "-package", "axis", "-type", "axis", "x", "y", "z", "v")
	generated, err := run.Output()
	if err != nil {
		t.Fatalf("the example fails: %v", err)
	}
	if formatted, err := format.Source(generated); err != nil || string(formatted) != string(generated) {
		t.Errorf("gofmt would change the generated file (%v):\n%s", err, generated)
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":         "module axis\n\ngo 1.24\n",
		"axis.go":        enum,
		"axis_string.go": string(generated),
		"axis_test.go":   enumTest,
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	test := exec.CommandContext(t.Context(), goTool, "test", "./...")
	test.Dir = dir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("the generated package fails its test: %v\n%s", err, out)
	}
}
