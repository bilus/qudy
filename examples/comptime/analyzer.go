package comptime

import (
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/token"
	"go/types"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

// verdictFact is the summary of one symbol that a package hands to its importers.
type verdictFact struct{ Verdict Verdict }

// AFact marks verdictFact as a fact of the analysis framework.
func (*verdictFact) AFact() {}

// String renders the fact for the framework's logs.
func (f *verdictFact) String() string { return f.Verdict.String() }

// NewAnalyzer returns the analysis that judges one package under the given rules.
//
// Its result is a *[Package]. It exports one fact per symbol that another
// package can name, so an importer reads verdicts and never bodies.
func NewAnalyzer(rules Rules) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:       "comptime",
		Doc:        "list the exported symbols a build-time program can evaluate ahead of time",
		Requires:   []*analysis.Analyzer{buildssa.Analyzer},
		FactTypes:  []analysis.Fact{new(verdictFact)},
		ResultType: reflect.TypeFor[*Package](),
		Run: func(pass *analysis.Pass) (any, error) {
			return newSolver(pass, rules).run(), nil
		},
	}
}

// solver computes the verdict of every declaration of one package.
type solver struct {
	pass     *analysis.Pass
	src      *Source
	order    []types.Object                        // the declarations, in source order
	judges   map[types.Object]func(Lookup) Verdict // how to judge each declaration
	symbols  map[types.Object]Kind                 // the declarations to report
	verdicts map[types.Object]Verdict
	visited  map[types.Object]bool // the declarations judged or under judgment in this round
	open     map[types.Object]bool // the declarations under judgment
	assumed  bool                  // whether this round used a verdict still under judgment
	changed  bool                  // whether this round changed a verdict
}

// newSolver collects the declarations of a package.
func newSolver(pass *analysis.Pass, rules Rules) *solver {
	s := &solver{
		pass:     pass,
		src:      newSource(pass),
		judges:   map[types.Object]func(Lookup) Verdict{},
		symbols:  map[types.Object]Kind{},
		verdicts: map[types.Object]Verdict{},
		open:     map[types.Object]bool{},
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				s.addGenDecl(decl, rules)
			case *ast.FuncDecl:
				s.addFunc(decl, rules)
			}
		}
	}
	return s
}

// add registers a declaration, the way to judge it, and the kind to report it under, if any.
func (s *solver) add(obj types.Object, kind Kind, judge func(Lookup) Verdict) {
	if obj == nil || s.judges[obj] != nil {
		return
	}
	s.order = append(s.order, obj)
	s.judges[obj] = judge
	if kind != "" {
		s.symbols[obj] = kind
	}
}

// addGenDecl registers the constants, variables and types of a declaration.
func (s *solver) addGenDecl(decl *ast.GenDecl, rules Rules) {
	var values *ast.ValueSpec // the last constant specification that has values
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			s.addType(spec, rules)
		case *ast.ValueSpec:
			if len(spec.Values) > 0 || values == nil {
				values = spec
			}
			s.addValues(spec, values, rules)
		}
	}
}

// addType registers a package-level type, which is judged and never reported.
func (s *solver) addType(spec *ast.TypeSpec, rules Rules) {
	obj, _ := s.pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
	if obj == nil {
		return
	}
	t := &TypeDecl{Source: s.src, Object: obj, Spec: spec}
	s.add(obj, "", func(look Lookup) Verdict { return rules.Type(t, look) })
}

// addValues registers the names of one constant or variable specification.
//
// A constant specification without values repeats the type and the values of
// the last one that has them.
func (s *solver) addValues(spec, values *ast.ValueSpec, rules Rules) {
	for i, name := range spec.Names {
		switch obj := s.pass.TypesInfo.Defs[name].(type) {
		case *types.Const:
			c := &Constant{Source: s.src, Object: obj, Type: values.Type}
			if i < len(values.Values) {
				c.Value = values.Values[i]
			}
			s.add(obj, exportedKind(obj, KindConst), func(look Lookup) Verdict { return rules.Const(c, look) })
		case *types.Var:
			v := &Variable{Source: s.src, Object: obj}
			s.add(obj, exportedKind(obj, KindVar), func(Lookup) Verdict { return rules.Var(v) })
		}
	}
}

// addFunc registers a function or a method with its SSA form.
func (s *solver) addFunc(decl *ast.FuncDecl, rules Rules) {
	obj, _ := s.pass.TypesInfo.Defs[decl.Name].(*types.Func)
	if obj == nil {
		return
	}
	ssa := s.pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	f := &Function{Source: s.src, Object: obj, Decl: decl, Body: ssa.Pkg.Prog.FuncValue(obj)}
	kind := exportedKind(obj, KindFunc)
	if recv := receiverName(obj); recv != "" {
		kind = exportedKind(obj, KindMethod)
		if !token.IsExported(recv) {
			kind = ""
		}
	}
	s.add(obj, kind, func(look Lookup) Verdict { return rules.Func(f, look) })
}

// exportedKind returns the kind to report a symbol under, or "" when its name is not exported.
func exportedKind(obj types.Object, kind Kind) Kind {
	if !obj.Exported() {
		return ""
	}
	return kind
}

// run judges every declaration, hands the verdicts to importers and returns the exported symbols.
//
// Recursion is solved as a greatest fixed point. A verdict that is asked for
// while it is under judgment counts as comptime without conditions, and the
// rounds repeat until none changes. The rules are monotone, so a verdict only
// ever gets worse and the rounds end. What is left is a set of functions in
// which each one uses only allowed constructs and calls only functions of the
// set, so no execution that starts in the set reaches a construct that is not
// allowed.
func (s *solver) run() *Package {
	for {
		s.visited = map[types.Object]bool{}
		s.assumed, s.changed = false, false
		for _, obj := range s.order {
			s.visit(obj)
		}
		if !s.assumed || !s.changed {
			break
		}
	}
	s.exportFacts()
	return s.result()
}

// visit judges a declaration once per round; a rejection is final.
func (s *solver) visit(obj types.Object) {
	if s.visited[obj] {
		return
	}
	s.visited[obj] = true
	old, judged := s.verdicts[obj]
	if judged && !old.Comptime {
		return
	}
	if !judged {
		s.verdicts[obj] = Accepted()
	}
	s.open[obj] = true
	v := worse(old, s.checked(obj, s.judges[obj](s.look)))
	delete(s.open, obj)
	if !judged || !v.equal(old) {
		s.changed = true
	}
	s.verdicts[obj] = v
}

// checked returns a verdict of the rules, or a rejection when the verdict contradicts itself.
func (s *solver) checked(obj types.Object, v Verdict) Verdict {
	if v.Comptime == (v.Reason == nil) {
		return v
	}
	return Rejected(&Reason{
		Kind:   ReasonUnknown,
		Pos:    s.src.At(obj.Pos()),
		Detail: "the rules gave a verdict that is neither comptime nor has a reason",
	})
}

// worse returns the new verdict with the conditions of the old one added.
//
// Monotone rules never drop a condition from one round to the next. Adding
// the old ones makes the rounds end for rules that are not monotone, too.
func worse(old, v Verdict) Verdict {
	if !v.Comptime {
		return v
	}
	return Accepted(slices.Concat(old.Conditions, v.Conditions)...)
}

// look returns the verdict of a symbol of this package or of a dependency.
func (s *solver) look(obj types.Object) Verdict {
	if obj.Pkg() != s.pass.Pkg {
		return s.imported(obj)
	}
	if s.judges[obj] == nil {
		return s.unknown(obj)
	}
	s.visit(obj)
	if s.open[obj] {
		s.assumed = true
	}
	return s.verdicts[obj]
}

// imported returns the verdict a dependency recorded for one of its symbols.
func (s *solver) imported(obj types.Object) Verdict {
	var fact verdictFact
	if s.pass.ImportObjectFact(obj, &fact) {
		return fact.Verdict
	}
	return s.unknown(obj)
}

// unknown returns the verdict of a symbol nobody judged.
func (s *solver) unknown(obj types.Object) Verdict {
	return Rejected(&Reason{
		Kind:   ReasonUnknown,
		Pos:    s.src.At(obj.Pos()),
		Cause:  SymbolName(obj),
		Detail: "no verdict was recorded for the symbol",
	})
}

// exportFacts hands importers the verdict of every symbol they can name.
func (s *solver) exportFacts() {
	for _, obj := range s.order {
		if obj.Exported() {
			s.pass.ExportObjectFact(obj, &verdictFact{s.verdicts[obj]})
		}
	}
}

// result lists the exported symbols of the package, sorted by name.
func (s *solver) result() *Package {
	pkg := &Package{Path: s.pass.Pkg.Path(), Symbols: []Symbol{}}
	for _, obj := range s.order {
		kind, ok := s.symbols[obj]
		if !ok {
			continue
		}
		name := strings.TrimPrefix(SymbolName(obj), pkg.Path+".")
		pkg.Symbols = append(pkg.Symbols, Symbol{Name: name, Kind: kind, Pos: s.src.At(obj.Pos()), Verdict: s.verdicts[obj]})
	}
	slices.SortFunc(pkg.Symbols, func(a, b Symbol) int { return strings.Compare(a.Name, b.Name) })
	return pkg
}

// newSource describes the files of a package: which ones a build constraint selects, and which ones cgo generated.
func newSource(pass *analysis.Pass) *Source {
	src := &Source{Fset: pass.Fset, Info: pass.TypesInfo, Constrained: map[*token.File]bool{}, Cgo: map[*token.File]bool{}}
	for _, file := range pass.Files {
		tf := pass.Fset.File(file.Package)
		if hasBuildLine(file) || nameConstrained(tf.Name()) {
			src.Constrained[tf] = true
		}
		if cgoGenerated(file) {
			src.Cgo[tf] = true
		}
	}
	return src
}

// cgoPrefixes are the prefixes cgo gives to the names it generates.
var cgoPrefixes = []string{"_Ctype_", "_Cfunc_", "_C2func_", "_Cvar_", "_Ciconst_", "_Cfconst_", "_Csconst_", "_Cmacro_"}

// cgoGenerated reports whether cgo wrote a file.
//
// Two signs each suffice, so that a change in one of them does not let the
// constants cgo took from C pass for plain Go: the notice cgo puts before the
// package clause, and a name with a prefix cgo gives to what it generates.
func cgoGenerated(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if group.Pos() < file.Package && c.Text == "// Code generated by cmd/cgo; DO NOT EDIT." {
				return true
			}
		}
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && hasCgoPrefix(id.Name) {
			found = true
		}
		return !found
	})
	return found
}

// hasCgoPrefix reports whether a name starts the way the names cgo generates do.
func hasCgoPrefix(name string) bool {
	return slices.ContainsFunc(cgoPrefixes, func(prefix string) bool { return strings.HasPrefix(name, prefix) })
}

// hasBuildLine reports whether a build constraint stands before the package clause.
func hasBuildLine(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				return true
			}
		}
	}
	return false
}

// nowhere is a build context that no operating system or architecture in a file name matches.
var nowhere = build.Context{
	GOOS:   "none",
	GOARCH: "none",
	OpenFile: func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("package p\n")), nil
	},
}

// nameConstrained reports whether the name of a file ties it to an operating system or an architecture.
func nameConstrained(name string) bool {
	match, err := nowhere.MatchFile("", filepath.Base(name))
	return err != nil || !match
}
