package gen

import (
	"go/ast"
	"go/parser"
	"go/types"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// A function labelled Comptime uses no symbol of another module outside the index.
// The uses are found with go/types, independently of the generator's bookkeeping.
func TestComptimeFunctionsUseOnlyTheIndex(t *testing.T) {
	trusted := Index()
	used := map[string]bool{}
	rapid.Check(t, func(t *rapid.T) {
		p := Generate(t, Default)
		labels := map[string]Label{}
		for _, s := range p.Symbols {
			labels[s.Package+"."+s.Name] = s.Label
		}
		checked := moduleImporter{}
		for _, k := range p.Packages {
			file, err := parser.ParseFile(fset, k.File, k.Src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
			pkg, err := (&types.Config{Importer: checked}).Check(k.Path, fset, []*ast.File{file}, info)
			if err != nil {
				t.Fatal(err)
			}
			checked[k.Path] = pkg
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				name := fn.Name.Name
				if fn.Recv != nil {
					name = receiver(fn) + "." + name
				}
				label, generated := labels[k.Path+"."+name]
				if !generated || label != Comptime {
					continue
				}
				ast.Inspect(fn, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					obj := info.Uses[id]
					if !ok || obj == nil || obj.Pkg() == nil || strings.HasPrefix(obj.Pkg().Path(), p.Module) {
						return true
					}
					symbol := obj.Pkg().Path() + "." + obj.Name()
					if !slices.Contains(trusted, symbol) {
						t.Fatalf("%s is labelled comptime and uses %s, which is not in the index\n%s", name, symbol, k.Src)
					}
					used[symbol] = true
					return true
				})
			}
		}
	})
	for _, symbol := range trusted {
		if !used[symbol] {
			t.Errorf("no comptime function used the trusted symbol %s", symbol)
		}
	}
}

// receiver returns the name of a method's receiver type.
func receiver(fn *ast.FuncDecl) string {
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	return typ.(*ast.Ident).Name
}
