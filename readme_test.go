package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// goBlocks matches the Go code blocks of the README.
var goBlocks = regexp.MustCompile("(?s)```go\n(.*?)```")

// readmeBlock returns the README's Go code block with the given first line.
func readmeBlock(t *testing.T, firstLine string) string {
	t.Helper()
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range goBlocks.FindAllStringSubmatch(string(readme), -1) {
		if strings.HasPrefix(block[1], firstLine+"\n") {
			return block[1]
		}
	}
	t.Fatalf("the README has no Go block that starts with %q", firstLine)
	return ""
}

func TestTheQuickStartWritesTheOutputItShows(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	gen, err := qudy.Compile("stringer/stringer.go", []byte(readmeBlock(t, "//go:build qudy")), "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":                   "module colors\n\ngo 1.25\n",
		"stringer/stringer_gen.go": string(gen),
	}
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := exec.CommandContext(t.Context(), goTool, "run", "./stringer", "colors", "Color", "Red", "Green", "Blue")
	run.Dir = dir
	got, err := run.Output()
	if err != nil {
		t.Fatalf("the quick start's generator fails: %v", err)
	}
	if want := readmeBlock(t, "package colors"); string(got) != want {
		t.Errorf("the quick start writes\n%s\nand the README shows\n%s", got, want)
	}
}
