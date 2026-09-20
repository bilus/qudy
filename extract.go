package qudy

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/txtar"
)

// Extract writes an archive's files under dir, gofmt-formatted, and returns the comment.
func Extract(archive []byte, dir string) ([]byte, error) {
	a := txtar.Parse(archive)
	if len(a.Files) == 0 {
		return nil, fmt.Errorf("the archive holds no files, and a file starts with a line such as -- name.go --")
	}
	for i, file := range a.Files {
		if !filepath.IsLocal(filepath.FromSlash(file.Name)) {
			return nil, fmt.Errorf("%s: the archive names a file outside the directory", file.Name)
		}
		if !strings.HasSuffix(file.Name, ".go") {
			continue
		}
		formatted, err := format.Source(file.Data)
		if err != nil {
			return nil, fmt.Errorf("%s: the file is not Go: %w", file.Name, err)
		}
		a.Files[i].Data = formatted
	}
	for _, file := range a.Files {
		path := filepath.Join(dir, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, file.Data, 0o644); err != nil {
			return nil, err
		}
	}
	return a.Comment, nil
}
