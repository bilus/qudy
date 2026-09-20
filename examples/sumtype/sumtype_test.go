//go:build go1.27

package sumtype_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bilus/qudy/examples/sumtype"
)

// program uses every generated declaration, including the zero values.
const program = `package shapes

import (
	"fmt"
	"testing"
)

func name(s Shape) string {
	return s.Match(
		func(c Circle) string { return fmt.Sprintf("circle %v", c.R) },
		func(r Rect) string { return fmt.Sprintf("rect %vx%v", r.W, r.H) },
	)
}

func kind(tok token) string {
	return tok.Match(
		func(w word) string { return "word " + string(w) },
		func(n number) string { return fmt.Sprintf("number %d", n) },
	)
}

func TestGenerated(t *testing.T) {
	got := []string{
		name(NewShape(Circle{R: 1})),
		name(NewShape(Rect{W: 2, H: 3})),
		name(Shape{}),
		kind(newToken(word("go"))),
		kind(newToken(number(7))),
		kind(token{}),
	}
	want := []string{"circle 1", "rect 2x3", "circle 0", "word go", "number 7", "word "}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %q, want %q", got[i], want[i])
		}
	}
}
`

func TestGenerateWritesCodeThatCompilesAndRuns(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	dir := t.TempDir()
	src, err := os.ReadFile("testdata/shapes/shapes.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":         "module shapes\n\ngo 1.27\n",
		"shapes.go":      string(src),
		"shapes_test.go": program,
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	written, err := sumtype.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "shapes_sumtype.go"); len(written) != 1 || written[0] != want {
		t.Fatalf("Generate wrote %v, want %s alone", written, want)
	}

	again, err := sumtype.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 {
		t.Fatalf("a second Generate wrote %v, want the same one file", again)
	}

	test := exec.CommandContext(t.Context(), goTool, "test", "./...")
	test.Dir = dir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("the generated package fails its test: %v\n%s", err, out)
	}
}

// writeFiles writes a package's files into a fresh directory.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGenerateSkipsGeneratedFiles(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"shapes.go":      "package shapes\n\n//sumtype: Shape, default=Circle\ntype Circle struct{}\n",
		"old_sumtype.go": "package shapes\n\nfunc (",
	})
	written, err := sumtype.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "shapes_sumtype.go"); len(written) != 1 || written[0] != want {
		t.Errorf("Generate wrote %v, want %s alone", written, want)
	}
}

func TestGenerateLeavesTestFilesAlone(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"shapes_test.go": "package shapes\n\n//sumtype: Shape, default=Circle\ntype Circle struct{}\n",
	})
	written, err := sumtype.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Errorf("Generate wrote %v, want nothing for a test file", written)
	}
}
