package qudy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

func TestExtractWritesTheFilesOfAnArchive(t *testing.T) {
	archive := "a note from the generator\n" +
		"-- shapes.go --\npackage shapes\nfunc  f( ) {\nreturn}\n" +
		"-- internal/data/names.txt --\n  kept as written\n"
	dir := t.TempDir()
	comment, err := qudy.Extract([]byte(archive), dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(comment) != "a note from the generator\n" {
		t.Errorf("the comment is %q", comment)
	}
	type testCase struct {
		description string
		file        string
		want        string
	}
	for _, c := range []testCase{
		{"a Go file comes out formatted", "shapes.go", "package shapes\n\nfunc f() {\n\treturn\n}\n"},
		{"another file keeps its text, in a new directory", "internal/data/names.txt", "  kept as written\n"},
	} {
		t.Run(c.description, func(t *testing.T) {
			got, err := os.ReadFile(filepath.Join(dir, c.file))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.want {
				t.Errorf("%s holds\n%s\nwant\n%s", c.file, got, c.want)
			}
		})
	}
}

func TestExtractRejects(t *testing.T) {
	type testCase struct {
		description string
		archive     string
		rejects     string
	}
	for _, c := range []testCase{
		{"an archive without a file", "package p\n", "the archive holds no files"},
		{"a file above the directory", "-- ok.go --\npackage p\n-- ../up.go --\npackage p\n", "../up.go: the archive names a file outside the directory"},
		{"a file with an absolute path", "-- /tmp/abs.go --\npackage p\n", "outside the directory"},
		{"a Go file that is not Go", "-- ok.txt --\nfine\n-- bad.go --\nfunc (\n", "bad.go: the file is not Go"},
	} {
		t.Run(c.description, func(t *testing.T) {
			dir := t.TempDir()
			_, err := qudy.Extract([]byte(c.archive), dir)
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Extract returned %v, want a rejection naming %q", err, c.rejects)
			}
			written, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(written) != 0 {
				t.Errorf("a rejected archive left %d entries behind", len(written))
			}
		})
	}
}
