package comptime

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"slices"
)

// FirstRules is the provisional first rule set, an allow-list over syntax and SSA instructions.
//
// Each rule below names what it accepts. Whatever it does not name is rejected
// with the construct as the reason.
type FirstRules struct{}

// judge collects the conditions of one verdict and the first reason against it.
type judge struct {
	src    *Source
	look   Lookup
	conds  []Condition
	reason *Reason
	at     token.Pos // the last position seen, for instructions that carry none
}

// verdict returns what the judge found.
func (j *judge) verdict() Verdict {
	if j.reason != nil {
		return Rejected(j.reason)
	}
	return Accepted(j.conds...)
}

// need records a condition of the verdict.
func (j *judge) need(c Condition) {
	if !slices.Contains(j.conds, c) {
		j.conds = append(j.conds, c)
	}
}

// reject records a reason against the symbol; the first reason wins.
func (j *judge) reject(kind ReasonKind, pos token.Pos, detail string) {
	if j.reason == nil {
		j.reason = &Reason{Kind: kind, Pos: j.src.At(pos), Detail: detail}
	}
}

// blame records a reason against the symbol that names the symbol causing it, if that one has a name.
func (j *judge) blame(kind ReasonKind, pos token.Pos, cause types.Object, detail string, why *Reason) {
	if j.reason != nil {
		return
	}
	j.reason = &Reason{Kind: kind, Pos: j.src.At(pos), Detail: detail, Why: why}
	if cause != nil {
		j.reason.Cause = SymbolName(cause)
	}
}

// depend makes the verdict depend on that of another symbol.
func (j *judge) depend(obj types.Object, pos token.Pos, detail string) {
	v := j.look(obj)
	if !v.Comptime {
		j.blame(ReasonDependency, pos, obj, detail, v.Reason)
		return
	}
	for _, c := range v.Conditions {
		j.need(c)
	}
}

// declared notes the file a declaration stands in.
//
// Rule: a declaration in a file that a build constraint or a file name selects
// holds only for hosts that select the same file. This narrows the given rule
// set; without it runtime.GOOS would be listed as comptime for every host.
//
// Rule: a declaration in a file that cgo generated is not comptime. Such a
// file holds the constants and types cgo took from the C headers of the
// target, or it is a Go file that imports "C". The rule rejects the plain Go
// code of the second kind too, which is more than it must.
func (j *judge) declared(pos token.Pos) {
	j.at = pos
	if j.src.IsCgo(pos) {
		j.reject(ReasonCgo, pos, "declaration in a file that cgo generated")
	}
	if j.src.IsConstrained(pos) {
		j.need(SameBuildConfig)
	}
}

// Const judges a package-level constant.
//
// Rule: a constant is comptime, since the compiler computes it. Its value
// depends on the target only through the constants and types it names, through
// the size of int, and through package unsafe; see [judge.constant].
// A constant that names a package-level variable, as in len(table), is
// rejected; that narrows the rule, to keep one rule for names.
func (FirstRules) Const(c *Constant, look Lookup) Verdict {
	j := &judge{src: c.Source, look: look}
	j.declared(c.Object.Pos())
	if c.Type != nil {
		j.syntax(c.Type)
	}
	if c.Value != nil {
		j.syntax(c.Value)
	}
	// The value itself is checked too: one expression gives several values under iota.
	if t := c.Object.Type(); sizedByTarget(t) && !fitsIn32Bits(c.Object.Val(), t) {
		j.need(SameIntSize)
	}
	return j.verdict()
}

// Var judges a package-level variable.
//
// Rule: a package-level variable is never comptime. Any package that sees it
// can assign to it, and its initial value is computed at run time.
func (FirstRules) Var(v *Variable) Verdict {
	j := &judge{src: v.Source}
	j.reject(ReasonVariable, v.Object.Pos(), "a package-level variable can change at run time")
	return j.verdict()
}

// Type judges a package-level type.
//
// Rule: a type is the same on host and target when the constants and types its
// declaration names are. The lengths of its arrays are the constants in question.
func (FirstRules) Type(t *TypeDecl, look Lookup) Verdict {
	j := &judge{src: t.Source, look: look}
	j.declared(t.Object.Pos())
	j.syntax(t.Spec)
	return j.verdict()
}

// Func judges a declared function or method.
//
// Rule: a function is comptime when it has a Go body, has no type parameters,
// names only comptime constants and types, and every SSA instruction of its
// body is on the allow-list of [judge.instruction].
//
// Generic functions are rejected: their instructions depend on the type
// arguments, and judging them needs a verdict per instantiation.
func (FirstRules) Func(f *Function, look Lookup) Verdict {
	j := &judge{src: f.Source, look: look}
	j.declared(f.Object.Pos())
	sig := f.Object.Signature()
	switch {
	case f.Decl.Body == nil:
		j.reject(ReasonNoBody, f.Object.Pos(), "the function has no Go body")
	case sig.TypeParams().Len() > 0 || sig.RecvTypeParams().Len() > 0:
		j.reject(ReasonGeneric, f.Object.Pos(), "the function has type parameters")
	case f.Body == nil || len(f.Body.Blocks) == 0:
		j.reject(ReasonUnsupported, f.Object.Pos(), "the function has no SSA form")
	case len(f.Body.AnonFuncs) > 0:
		j.reject(ReasonFuncValue, f.Body.AnonFuncs[0].Pos(), "function literal")
	default:
		j.syntax(f.Decl)
		j.body(f)
	}
	return j.verdict()
}

// syntax checks every name and constant expression under a node.
func (j *judge) syntax(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			j.name(id)
		}
		if e, ok := n.(ast.Expr); ok {
			j.constant(e)
		}
		return j.reason == nil
	})
}

// name checks a use of a name.
//
// Rule: the names of locals, fields, methods and the universe are allowed here;
// the instructions that use them are judged on their own. A package-level
// constant or type must be comptime itself. A package-level variable is not
// allowed, read or written. Nothing of package unsafe is allowed, not even
// the name unsafe.Pointer in the declaration of a type.
func (j *judge) name(id *ast.Ident) {
	obj := j.src.Info.Uses[id]
	if obj == nil || obj.Pkg() == nil {
		return
	}
	if obj.Pkg().Path() == "unsafe" {
		j.reject(ReasonUnsafe, id.Pos(), "unsafe."+obj.Name())
		return
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return
	}
	switch obj := obj.(type) {
	case *types.Const:
		j.depend(obj, id.Pos(), "constant")
	case *types.TypeName:
		j.depend(obj, id.Pos(), "type")
	case *types.Var:
		j.blame(ReasonVariable, id.Pos(), obj, "reference to a package-level variable", nil)
	}
}

// constant checks a constant expression of type int, uint or uintptr.
//
// Rule: constant arithmetic is exact, and a value that overflows its type
// does not compile. So the size of int reaches a constant expression in two
// ways. One is ^x, which flips as many bits as the type has. The other is a
// value that needs more than 32 bits, which compiles only where int has 64.
func (j *judge) constant(e ast.Expr) {
	tv := j.src.Info.Types[e]
	if tv.Value == nil || tv.Type == nil || !sizedByTarget(tv.Type) {
		return
	}
	if unary, ok := e.(*ast.UnaryExpr); ok && unary.Op == token.XOR {
		j.need(SameIntSize)
	}
	if !fitsIn32Bits(tv.Value, tv.Type) {
		j.need(SameIntSize)
	}
}

// fitsIn32Bits reports whether an integer constant fits its type when the type has 32 bits.
func fitsIn32Bits(v constant.Value, t types.Type) bool {
	v = constant.ToInt(v)
	if v.Kind() != constant.Int {
		return false
	}
	low, high := int64(0), int64(math.MaxUint32)
	if t.Underlying().(*types.Basic).Kind() == types.Int {
		low, high = math.MinInt32, math.MaxInt32
	}
	return constant.Compare(v, token.GEQ, constant.MakeInt64(low)) && constant.Compare(v, token.LEQ, constant.MakeInt64(high))
}

// sizedByTarget reports whether a type is int, uint or uintptr underneath.
func sizedByTarget(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && (b.Kind() == types.Int || b.Kind() == types.Uint || b.Kind() == types.Uintptr)
}
