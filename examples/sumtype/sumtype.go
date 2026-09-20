//go:build go1.27

// Package sumtype generates the boilerplate for a package's sum types.
package sumtype

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// generatedSuffix ends the name of every generated file.
const generatedSuffix = "_sumtype.go"

// Render returns a source file's generated file, or nil without a sum type.
func Render(filename string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	sums, err := findSums(fset, file)
	if err != nil || len(sums) == 0 {
		return nil, err
	}
	g := newWriter()
	g.writeHeader(filepath.Base(filename), file.Name.Name)
	for _, sum := range sums {
		g.writeSum(sum)
	}
	out, err := format.Source([]byte(g.out.String()))
	if err != nil {
		return nil, fmt.Errorf("%s: the generated file is not Go: %w", filename, err)
	}
	return out, nil
}

// Generate writes the generated file of each source file in dir.
func Generate(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isSourceFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out, err := Render(path, src)
		if err != nil {
			return nil, err
		}
		if out == nil {
			continue
		}
		target := strings.TrimSuffix(path, ".go") + generatedSuffix
		if err := os.WriteFile(target, out, 0o644); err != nil {
			return nil, err
		}
		written = append(written, target)
	}
	return written, nil
}

// isSourceFile reports whether a file can declare a sum type.
func isSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, generatedSuffix)
}
