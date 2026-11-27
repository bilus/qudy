package comptime

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Rules is the definition of comptime, kept apart from loading, ordering and output so it can be replaced.
//
// Rules are an allow-list: a rule accepts a symbol only when it can prove the
// symbol comptime, and rejects everything it does not recognize.
//
// A rule must be monotone in look: a worse verdict for a symbol it looks up
// never gives a better verdict for the symbol it judges. The engine relies on
// that to resolve recursion. Packages are judged in parallel, so a rule must
// be safe to call from several goroutines.
type Rules interface {
	// Const judges a package-level constant.
	Const(c *Constant, look Lookup) Verdict
	// Var judges a package-level variable.
	Var(v *Variable) Verdict
	// Type judges a package-level type: whether it is the same type on host and target.
	Type(t *TypeDecl, look Lookup) Verdict
	// Func judges a declared function or method.
	Func(f *Function, look Lookup) Verdict
}

// Lookup returns the verdict of a package-level symbol or a method, from this package or another.
type Lookup func(types.Object) Verdict

// Source is the type-checked package a declaration comes from.
type Source struct {
	Fset        *token.FileSet
	Info        *types.Info
	Constrained map[*token.File]bool // the files a build constraint or a file name selects
	Cgo         map[*token.File]bool // the files cgo generated
}

// IsCgo reports whether a position lies in a file that cgo generated.
func (s *Source) IsCgo(pos token.Pos) bool {
	return s.Cgo[s.Fset.File(pos)]
}

// IsConstrained reports whether a position lies in a file that a build constraint or its name selects.
func (s *Source) IsConstrained(pos token.Pos) bool {
	return s.Constrained[s.Fset.File(pos)]
}

// At renders a position as file:line:column.
func (s *Source) At(pos token.Pos) string {
	return s.Fset.Position(pos).String()
}

// Constant is a package-level constant handed to the rules.
type Constant struct {
	*Source
	Object *types.Const
	Type   ast.Expr // the declared type, or nil
	Value  ast.Expr // the expression that gives the constant its value, or nil
}

// Variable is a package-level variable handed to the rules.
type Variable struct {
	*Source
	Object *types.Var
}

// TypeDecl is a package-level type declaration handed to the rules.
type TypeDecl struct {
	*Source
	Object *types.TypeName
	Spec   *ast.TypeSpec
}

// Function is a declared function or method handed to the rules.
type Function struct {
	*Source
	Object *types.Func
	Decl   *ast.FuncDecl
	Body   *ssa.Function // the SSA form, or nil when there is none
}

// SymbolName names a package-level symbol as path.Name, and a method as path.Type.Method.
func SymbolName(obj types.Object) string {
	name := obj.Name()
	if fn, ok := obj.(*types.Func); ok {
		if recv := receiverName(fn); recv != "" {
			name = recv + "." + name
		}
	}
	if obj.Pkg() == nil {
		return name
	}
	return obj.Pkg().Path() + "." + name
}

// receiverName returns the name of the type a method is declared on, or "" for a function.
func receiverName(fn *types.Func) string {
	recv := fn.Signature().Recv()
	if recv == nil {
		return ""
	}
	t := types.Unalias(recv.Type())
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return t.String()
}
