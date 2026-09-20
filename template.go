package qudy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
	declared     map[string]bool
	builtinCalls []builtinCall
	ownGensyms   ownGensyms
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
		declared:     declaredNames(file),
		builtinCalls: findBuiltinCalls(fset, file),
		ownGensyms:   findOwnGensyms(fset, file),
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

// declaredNames collects every name with a declaration in the template.
func declaredNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	add := func(idents ...*ast.Ident) {
		for _, id := range idents {
			if id != nil {
				names[id.Name] = true
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
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
		case *ast.Field:
			add(n.Names...)
		case *ast.RangeStmt:
			if id, ok := n.Key.(*ast.Ident); ok {
				add(id)
			}
			if id, ok := n.Value.(*ast.Ident); ok {
				add(id)
			}
		case *ast.FuncDecl:
			add(n.Name)
		case *ast.TypeSpec:
			add(n.Name)
		case *ast.LabeledStmt:
			add(n.Label)
		}
		return true
	})
	return names
}

// ownGensyms is where a template declares its own symbol generator.
type ownGensyms struct {
	packageLevel bool
	lines        []int
}

// newOwnGensyms returns the declarations at package level and on the given lines.
func newOwnGensyms(packageLevel bool, lines []int) ownGensyms {
	return ownGensyms{packageLevel: packageLevel, lines: lines}
}

// declaredIn reports whether the template declares a symbol generator for a function body.
func (o ownGensyms) declaredIn(body span) bool {
	if o.packageLevel {
		return true
	}
	for _, line := range o.lines {
		if body.from <= line && line <= body.to {
			return true
		}
	}
	return false
}

// findOwnGensyms finds every declaration of the symbol generator in the template.
func findOwnGensyms(fset *token.FileSet, file *ast.File) ownGensyms {
	packageLevel := false
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			packageLevel = packageLevel || d.Name.Name == gensymVariable
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && names(vs.Names, gensymVariable) {
					packageLevel = true
				}
			}
		}
	}
	var lines []int
	add := func(pos token.Pos) { lines = append(lines, fset.Position(pos).Line) }
	param := func(typ *ast.FuncType, body *ast.BlockStmt) {
		if body == nil {
			return
		}
		for _, field := range typ.Params.List {
			if names(field.Names, gensymVariable) {
				add(body.Lbrace)
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			param(n.Type, n.Body)
		case *ast.FuncLit:
			param(n.Type, n.Body)
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && n.Tok == token.DEFINE && id.Name == gensymVariable {
					add(n.Pos())
				}
			}
		case *ast.ValueSpec:
			if names(n.Names, gensymVariable) {
				add(n.Pos())
			}
		}
		return true
	})
	return newOwnGensyms(packageLevel, lines)
}

// names reports whether a list of identifiers holds a name.
func names(idents []*ast.Ident, name string) bool {
	for _, id := range idents {
		if id.Name == name {
			return true
		}
	}
	return false
}
