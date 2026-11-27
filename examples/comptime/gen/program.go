package gen

import (
	"os"
	"path/filepath"
)

// Label is what the generator knows, by construction, about a function it wrote.
type Label int

const (
	// Comptime marks a function built only from the comptime fragment, the
	// trusted symbols of the index, and other Comptime functions. A function
	// that calls its function or interface parameters is Comptime when the
	// rest of its body is: it is comptime relative to those parameters.
	Comptime Label = iota
	// Unknown marks a function that refers to a function that is not
	// Comptime, perhaps on a path that never runs.
	Unknown
	// Impure marks a function with a planted impurity. Apart from it the body
	// is comptime and cannot panic, so the impurity decides the result of
	// every call the generated main function makes.
	Impure
)

// String returns the label's name.
func (l Label) String() string { return [...]string{"comptime", "unknown", "impure"}[l] }

// Symbol is a generated function or method with its label.
type Symbol struct {
	Package  string   // import path
	Name     string   // "F3", or "S1.M4" for a method
	Exported bool     // an unexported function is reachable only through its callers
	Label    Label    // what is known about it
	Because  []string // for Unknown and Impure: the functions that lead to an impurity, then its name
	Impurity string   // for Impure: the impurity its result depends on
	Layers   []string // for Impure: the indirections between the body and a planted catalog impurity
	Shows    string   // for Impure: the condition that exposes it, or "" when none does reliably
	Features []string // the constructs the body uses
}

// Package is one package of a generated program.
type Package struct {
	Path string // import path
	Name string
	File string // path of its one source file below the module root
	Src  string
}

// Program is a generated module and what is known about its functions.
type Program struct {
	Module   string
	Packages []Package // in dependency order; the last is package main
	Symbols  []Symbol  // in declaration order
}

// Files returns the module's files by path below the module root, with its go.mod.
func (p Program) Files() map[string]string {
	files := map[string]string{"go.mod": "module " + p.Module + "\n\ngo 1.25\n"}
	for _, k := range p.Packages {
		files[k.File] = k.Src
	}
	return files
}

// Write writes the module's files below dir.
func (p Program) Write(dir string) error {
	for name, src := range p.Files() {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			return err
		}
	}
	return nil
}
