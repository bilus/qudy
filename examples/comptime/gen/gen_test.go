package gen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

var (
	fset = token.NewFileSet()
	std  = importer.ForCompiler(fset, "source", nil)
)

// moduleImporter resolves the packages of a generated module, and the standard library.
type moduleImporter map[string]*types.Package

// Import returns a package of the module, or of the standard library.
func (m moduleImporter) Import(path string) (*types.Package, error) {
	if p, ok := m[path]; ok {
		return p, nil
	}
	return std.Import(path)
}

// typecheck type-checks every package of a program, in order.
func typecheck(p Program) error {
	checked := moduleImporter{}
	for _, k := range p.Packages {
		file, err := parser.ParseFile(fset, k.File, k.Src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkg, err := (&types.Config{Importer: checked}).Check(k.Path, fset, []*ast.File{file}, nil)
		if err != nil {
			return fmt.Errorf("%w\n%s", err, k.Src)
		}
		checked[k.Path] = pkg
	}
	return nil
}

// lines counts the lines of a program's source.
func lines(p Program) (n int) {
	for _, k := range p.Packages {
		n += strings.Count(k.Src, "\n")
	}
	return n
}

// Every generated program type-checks, and its labels are consistent.
func TestProgramsTypeCheck(t *testing.T) {
	for name, cfg := range map[string]Config{"default": Default, "pure": Pure} {
		t.Run(name, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				p := Generate(t, cfg)
				if err := typecheck(p); err != nil {
					t.Fatal(err)
				}
				impure := 0
				for _, s := range p.Symbols {
					if s.Label == Impure {
						impure++
					}
					if cfg.Impurities == 0 && s.Label != Comptime {
						t.Fatalf("%s is %v in a program without impurities", s.Name, s.Label)
					}
					if (s.Label == Comptime) != (len(s.Because) == 0) {
						t.Fatalf("%s is %v because %v", s.Name, s.Label, s.Because)
					}
				}
				if cfg.Impurities > 0 && (impure < 1 || impure > cfg.Impurities) {
					t.Fatalf("%d functions are impure, and the configuration asks for 1 to %d", impure, cfg.Impurities)
				}
			})
		})
	}
}

// One seed gives one program.
func TestExampleIsDeterministic(t *testing.T) {
	for seed := range 20 {
		a, b := Example(seed, Default), Example(seed, Default)
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatalf("seed %d gave two different programs", seed)
		}
	}
}

// What do the programs look like? Sizes, labels, and how often each construct appears.
func TestStatistics(t *testing.T) {
	const n = 200
	var sizes, funcs []int
	labels, features, impurities, layers := map[Label]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	pkgs := 0
	for seed := range n {
		p := Example(seed, Default)
		sizes, funcs, pkgs = append(sizes, lines(p)), append(funcs, len(p.Symbols)), pkgs+len(p.Packages)
		seen := map[string]bool{}
		for _, s := range p.Symbols {
			labels[s.Label]++
			for _, f := range s.Features {
				seen[f] = true
			}
			if s.Impurity != "" {
				impurities[s.Impurity]++
			}
			for _, l := range s.Layers {
				layers[l]++
			}
		}
		for f := range seen {
			features[f]++
		}
	}
	sort.Ints(sizes)
	sort.Ints(funcs)
	t.Logf("%d programs: lines min %d, median %d, max %d; functions median %d, max %d; packages %.1f on average",
		n, sizes[0], sizes[n/2], sizes[n-1], funcs[n/2], funcs[n-1], float64(pkgs)/n)
	t.Logf("labels: %d comptime, %d unknown, %d impure", labels[Comptime], labels[Unknown], labels[Impure])
	t.Logf("programs that use each construct, of %d:\n%s", n, table(features))
	t.Logf("planted impurities:\n%s", table(impurities))
	t.Logf("indirections:\n%s", table(layers))
	for _, imp := range impurities2() {
		if impurities[imp] == 0 {
			t.Errorf("no program planted the impurity %q", imp)
		}
	}
}

// impurities2 returns the names of every impurity the generator can plant.
func impurities2() []string {
	names := []string{parameterWrite}
	for _, imp := range impurities {
		names = append(names, imp.name)
	}
	return names
}

// table formats counts, largest first.
func table(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "  %4d  %s\n", counts[name], name)
	}
	return b.String()
}
