package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

func TestGeneratorIsUpToDate(t *testing.T) {
	src, err := os.ReadFile("write.go")
	if err != nil {
		t.Fatal(err)
	}
	want, err := qudy.Compile("write.go", src, "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	checkedIn, err := os.ReadFile("write_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	_, got, found := strings.Cut(string(checkedIn), "DO NOT EDIT.\n\n")
	if !found || got != string(want) {
		t.Errorf("write_gen.go is stale, run go generate:\n%s\nwant\n%s", got, want)
	}
}

// enums declares an enum whose constants have the plain names of the gensyms.
const enums = `package units

type Unit int

const (
	s Unit = iota
	v
	m
)

type Color int

const (
	Red Color = iota
	Green
	Blue
)
`

// enumsTest parses the name of every constant back into the constant.
const enumsTest = `package units

import "testing"

func TestRoundTrip(t *testing.T) {
	for _, unit := range []Unit{s, v, m} {
		if got, err := ParseUnit(unit.String()); err != nil || got != unit {
			t.Errorf("ParseUnit(%q) returned %v, %v", unit.String(), got, err)
		}
	}
	if got, err := ParseColor("Green"); err != nil || got != Green {
		t.Errorf("ParseColor returned %v, %v", got, err)
	}
	if _, err := ParseUnit("kg"); err == nil {
		t.Error("ParseUnit accepted kg")
	}
}
`

func TestGeneratedFilesBuildInOnePackage(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	run := exec.CommandContext(t.Context(), goTool, "run", ".", "-package", "units", "Unit=s,v,m", "Color=Red,Green,Blue")
	archive, err := run.Output()
	if err != nil {
		t.Fatalf("the example fails: %v", err)
	}
	dir := t.TempDir()
	extract := exec.CommandContext(t.Context(), goTool, "run", "../../cmd/qudy", "-txtar", "-o", dir)
	extract.Stdin = bytes.NewReader(archive)
	if out, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("qudy -txtar fails: %v\n%s", err, out)
	}
	for name, want := range map[string]string{
		"unit_string.go":  "func (v_qd1 Unit) String() string {",
		"unit_parse.go":   "func ParseUnit(s_qd1 string) (Unit, error) {",
		"color_string.go": "func (v_qd1 Color) String() string {",
		"color_parse.go":  "func ParseColor(s_qd1 string) (Color, error) {",
	} {
		text, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("qudy -txtar did not write %s: %v", name, err)
		}
		if !strings.Contains(string(text), want) {
			t.Errorf("%s holds\n%s\nwant it to hold\n%s", name, text, want)
		}
	}
	files := map[string]string{
		"go.mod":        "module units\n\ngo 1.25\n",
		"units.go":      enums,
		"units_test.go": enumsTest,
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
