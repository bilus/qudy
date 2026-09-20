//go:build go1.27

package sumtype

import (
	"fmt"
	"go/ast"
	"go/token"
)

// findSums returns a file's sum types, in source order.
func findSums(fset *token.FileSet, file *ast.File) ([]sumType, error) {
	var sums []sumType
	found := map[token.Pos]bool{}
	for i, decl := range file.Decls {
		types, ok := typeDecl(decl)
		if !ok {
			continue
		}
		d, at, ok, err := declDirective(types)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fset.Position(at), err)
		}
		if !ok {
			continue
		}
		found[at] = true
		specs := types.Specs
		if !types.Lparen.IsValid() {
			specs = append(specs[:1:1], followingSpecs(fset, types, file.Decls[i+1:])...)
		}
		variants, err := variantNames(fset, specs)
		if err != nil {
			return nil, err
		}
		sum, err := newSumType(d, variants)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fset.Position(at), err)
		}
		sums = append(sums, sum)
	}
	if err := rejectMisplacedDirectives(fset, file, found); err != nil {
		return nil, err
	}
	return sums, rejectRedeclared(sums)
}

// typeDecl casts a declaration as a type declaration.
func typeDecl(decl ast.Decl) (*ast.GenDecl, bool) {
	gen, ok := decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.TYPE {
		return nil, false
	}
	return gen, true
}

// declDirective returns the directive in a declaration's doc comment, with its position.
func declDirective(decl *ast.GenDecl) (directive, token.Pos, bool, error) {
	if decl.Doc == nil {
		return directive{}, token.NoPos, false, nil
	}
	for _, c := range decl.Doc.List {
		if d, ok, err := parseDirective(c.Text); ok {
			return d, c.Pos(), true, err
		}
	}
	return directive{}, token.NoPos, false, nil
}

// followingSpecs detects the types declared on the lines right after a declaration.
func followingSpecs(fset *token.FileSet, first *ast.GenDecl, rest []ast.Decl) []ast.Spec {
	var specs []ast.Spec
	end := fset.Position(first.End()).Line
	for _, decl := range rest {
		types, ok := typeDecl(decl)
		if !ok || types.Lparen.IsValid() || fset.Position(declStart(types)).Line != end+1 {
			break
		}
		if _, _, ok, _ := declDirective(types); ok {
			// The next sum type starts here, and findSums reports its errors.
			break
		}
		specs = append(specs, types.Specs...)
		end = fset.Position(types.End()).Line
	}
	return specs
}

// declStart returns the declaration's start position, including doc comment.
func declStart(decl *ast.GenDecl) token.Pos {
	if decl.Doc != nil {
		return decl.Doc.Pos()
	}
	return decl.Pos()
}

// variantNames names the variants and rejects aliases, interfaces and generic types.
func variantNames(fset *token.FileSet, specs []ast.Spec) ([]string, error) {
	var names []string
	for _, spec := range specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		at := fset.Position(ts.Pos())
		switch _, isInterface := ts.Type.(*ast.InterfaceType); {
		case ts.Assign.IsValid():
			return nil, fmt.Errorf("%s: %s is an alias, and a variant is a defined type", at, ts.Name.Name)
		case ts.TypeParams != nil:
			return nil, fmt.Errorf("%s: %s has type parameters, and a variant has none", at, ts.Name.Name)
		case isInterface:
			return nil, fmt.Errorf("%s: %s is an interface, and a variant is a concrete type", at, ts.Name.Name)
		}
		names = append(names, ts.Name.Name)
	}
	return names, nil
}

// rejectMisplacedDirectives returns an error for a directive outside a type declaration's doc comment.
func rejectMisplacedDirectives(fset *token.FileSet, file *ast.File, found map[token.Pos]bool) error {
	for _, group := range file.Comments {
		for _, c := range group.List {
			// A malformed directive counts as misplaced too.
			if _, ok, _ := parseDirective(c.Text); ok && !found[c.Pos()] {
				return fmt.Errorf("%s: misplaced directive: it belongs in the doc comment of its first variant", fset.Position(c.Pos()))
			}
		}
	}
	return nil
}

// rejectRedeclared returns an error for a sum type redeclared in the file.
func rejectRedeclared(sums []sumType) error {
	seen := map[string]bool{}
	for _, s := range sums {
		if seen[s.name] {
			return fmt.Errorf("%s redeclared in this file", s.name)
		}
		seen[s.name] = true
	}
	return nil
}
