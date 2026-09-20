package qudy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// template is a parsed template.
type template struct {
	filename     string
	packageLine  int
	lines        []string
	outputLines  map[int]string
	blocks       []span
	funcBodies   []span
	importsFmt   bool
	declared     map[string]bool
	ownGensym    map[span]bool
	builtinCalls []builtinCall
}

// newTemplate parses a template and rejects misplaced output lines and GENSYM calls.
func newTemplate(filename string, src []byte) (*template, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: the template is not Go: %w", filename, err)
	}
	t := &template{
		filename:     filename,
		packageLine:  fset.Position(file.Package).Line,
		lines:        splitLines(string(src)),
		outputLines:  map[int]string{},
		blocks:       blockSpans(fset, file),
		funcBodies:   funcBodySpans(fset, file),
		importsFmt:   importsPackage(file, "fmt"),
		declared:     declaredNames(file),
		ownGensym:    ownGensymBodies(fset, file),
		builtinCalls: findBuiltinCalls(fset, file),
	}
	if name, ok := reservedName(t.declared); ok {
		return nil, fmt.Errorf("%s: the template declares %s, and qudy reserves names that end in %s", filename, name, gensymSuffix)
	}
	if t.declared[gensymBuiltin] {
		return nil, fmt.Errorf("%s: the template declares %s, which is a built-in function", filename, gensymBuiltin)
	}
	for _, c := range t.builtinCalls {
		if t.funcBodyOf(c.line) == (span{}) {
			return nil, fmt.Errorf("%s:%d: a %s call belongs inside a function body", filename, c.line, c.name)
		}
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			text, ok := outputText(c.Text)
			if !ok {
				continue
			}
			at := fset.Position(c.Slash)
			if strings.TrimSpace(t.lines[at.Line-1][:at.Column-1]) != "" {
				return nil, fmt.Errorf("%s:%d: an output line stands alone on its line, and code precedes this one", filename, at.Line)
			}
			if t.funcBodyOf(at.Line) == (span{}) {
				return nil, fmt.Errorf("%s:%d: an output line belongs inside a function body", filename, at.Line)
			}
			t.outputLines[at.Line] = text
		}
	}
	return t, nil
}

// builtinCall is one call to a built-in function, with its 1-based position.
type builtinCall struct {
	name     string
	line     int
	from, to int
}

// gensymBuiltin is the name of the built-in function that returns a gensym.
const gensymBuiltin = "GENSYM"

// builtinCallsOn returns the calls to built-in functions on one line.
func (t *template) builtinCallsOn(line int) []builtinCall {
	var calls []builtinCall
	for _, c := range t.builtinCalls {
		if c.line == line {
			calls = append(calls, c)
		}
	}
	return calls
}

// findBuiltinCalls finds every call to a built-in function in the file.
func findBuiltinCalls(fset *token.FileSet, file *ast.File) []builtinCall {
	var calls []builtinCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != gensymBuiltin {
			return true
		}
		at := fset.Position(id.Pos())
		calls = append(calls, builtinCall{name: id.Name, line: at.Line, from: at.Column, to: at.Column + len(id.Name)})
		return true
	})
	return calls
}

// span is a range of lines that includes both ends.
type span struct{ from, to int }

// blockOf returns the innermost block that holds a line, or the zero span.
func (t *template) blockOf(line int) span {
	return innermost(t.blocks, line)
}

// funcBodyOf returns the innermost function body that holds a line, or the zero span.
func (t *template) funcBodyOf(line int) span {
	return innermost(t.funcBodies, line)
}

// outerFuncBodyOf returns the outermost function body that holds a line, or the zero span.
func (t *template) outerFuncBodyOf(line int) span {
	var found span
	for _, s := range t.funcBodies {
		if s.from <= line && line <= s.to && (found == (span{}) || s.from < found.from) {
			found = s
		}
	}
	return found
}

// innermost returns the smallest span that holds a line, or the zero span.
func innermost(spans []span, line int) span {
	var found span
	for _, s := range spans {
		if s.from > line || line > s.to {
			continue
		}
		if found == (span{}) || s.from >= found.from && s.to <= found.to {
			found = s
		}
	}
	return found
}

// funcBodySpans returns the span of every function body in the file.
func funcBodySpans(fset *token.FileSet, file *ast.File) []span {
	var spans []span
	add := func(body *ast.BlockStmt) {
		if body != nil {
			spans = append(spans, span{fset.Position(body.Lbrace).Line, fset.Position(body.Rbrace).Line})
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			add(n.Body)
		case *ast.FuncLit:
			add(n.Body)
		}
		return true
	})
	return spans
}

// blockSpans returns the span of every block, including switch and select clauses.
func blockSpans(fset *token.FileSet, file *ast.File) []span {
	var spans []span
	add := func(from, to token.Pos) {
		spans = append(spans, span{fset.Position(from).Line, fset.Position(to).Line})
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BlockStmt:
			add(n.Lbrace, n.Rbrace)
		case *ast.CaseClause:
			if len(n.Body) > 0 {
				add(n.Colon, n.Body[len(n.Body)-1].End())
			}
		case *ast.CommClause:
			if len(n.Body) > 0 {
				add(n.Colon, n.Body[len(n.Body)-1].End())
			}
		}
		return true
	})
	return spans
}

// declaredNames collects every name that the template declares, except methods.
func declaredNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	add := func(idents ...*ast.Ident) {
		for _, id := range idents {
			names[id.Name] = true
		}
	}
	fields := func(lists ...*ast.FieldList) {
		for _, list := range lists {
			if list == nil {
				continue
			}
			for _, field := range list.List {
				add(field.Names...)
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Recv == nil {
				add(n.Name)
			}
			fields(n.Recv, n.Type.Params, n.Type.Results)
		case *ast.FuncLit:
			fields(n.Type.Params, n.Type.Results)
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, lhs := range n.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						add(id)
					}
				}
			}
		case *ast.ValueSpec:
			add(n.Names...)
		case *ast.TypeSpec:
			add(n.Name)
		}
		return true
	})
	return names
}

// reservedName returns a declared name that ends in the suffix of qudy's own names.
func reservedName(declared map[string]bool) (string, bool) {
	var reserved []string
	for name := range declared {
		if strings.HasSuffix(name, gensymSuffix) {
			reserved = append(reserved, name)
		}
	}
	if len(reserved) == 0 {
		return "", false
	}
	return slices.Min(reserved), true
}

// ownGensymBodies returns the function bodies for which the template declares qudyGensym.
func ownGensymBodies(fset *token.FileSet, file *ast.File) map[span]bool {
	own := map[span]bool{}
	everywhere := declaresInPackage(file, gensymVariable)
	ast.Inspect(file, func(n ast.Node) bool {
		var typ *ast.FuncType
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			typ, body = n.Type, n.Body
		case *ast.FuncLit:
			typ, body = n.Type, n.Body
		}
		if body != nil && (everywhere || declaresInFunc(typ, body, gensymVariable)) {
			own[span{fset.Position(body.Lbrace).Line, fset.Position(body.Rbrace).Line}] = true
		}
		return true
	})
	return own
}

// declaresInPackage reports whether the package declares a function or a variable with a name.
func declaresInPackage(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == name {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && hasName(vs.Names, name) {
					return true
				}
			}
		}
	}
	return false
}

// declaresInFunc reports whether a name is a parameter or a local of a function.
func declaresInFunc(typ *ast.FuncType, body *ast.BlockStmt, name string) bool {
	found := hasParam(typ, name)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			found = found || hasParam(n.Type, name)
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && n.Tok == token.DEFINE && id.Name == name {
					found = true
				}
			}
		case *ast.ValueSpec:
			found = found || hasName(n.Names, name)
		}
		return !found
	})
	return found
}

// hasParam reports whether a function type has a parameter with a name.
func hasParam(typ *ast.FuncType, name string) bool {
	for _, field := range typ.Params.List {
		if hasName(field.Names, name) {
			return true
		}
	}
	return false
}

// hasName reports whether a list of identifiers holds a name.
func hasName(idents []*ast.Ident, name string) bool {
	for _, id := range idents {
		if id.Name == name {
			return true
		}
	}
	return false
}

// importsPackage reports whether the file imports a standard package under its own name.
func importsPackage(file *ast.File, path string) bool {
	for _, spec := range file.Imports {
		if spec.Path.Value == strconv.Quote(path) && (spec.Name == nil || spec.Name.Name == path) {
			return true
		}
	}
	return false
}

// needsFmt reports whether the emit calls use fmt without an import in the template.
func (t *template) needsFmt(emit string) bool {
	return strings.HasPrefix(emit, "fmt.") && len(t.outputLines) > 0 && !t.importsFmt
}
