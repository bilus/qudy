package qudy_test

import (
	"go/format"
	"os"
	"testing"

	"github.com/bilus/qudy"
)

// TestCompileTheEntryTemplate compiles livegen's dispatch entry against its golden file.
func TestCompileTheEntryTemplate(t *testing.T) {
	src, err := os.ReadFile("testdata/entry.go")
	if err != nil {
		t.Fatal(err)
	}
	gen, err := qudy.Compile("entry.go", src, "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/entry.golden")
	if err != nil {
		t.Fatal(err)
	}
	if string(gen) != string(want) {
		t.Errorf("compiled to\n%s\nwant\n%s", gen, want)
	}
}

// TestGofmtKeepsTheEntryTemplate checks that gofmt leaves the template unchanged, including its output lines.
func TestGofmtKeepsTheEntryTemplate(t *testing.T) {
	src, err := os.ReadFile("testdata/entry.go")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != string(src) {
		t.Errorf("gofmt changed the template:\n%s", formatted)
	}
}
