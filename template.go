package qudy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// template is a parsed template file.
type template struct {
	filename string
	lines    []string
	outputs  map[int]string
	blocks   []span
	funcs    []span
	declared map[string]bool
	builtins []builtin
}

// newTemplate parses a template and rejects an output line outside a function body.
func newTemplate(filename string, src []byte) (*template, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: the template is not Go: %w", filename, err)
	}
	t := &template{
		filename: filename,
		lines:    splitLines(string(src)),
		outputs:  map[int]string{},
		blocks:   blockSpans(fset, file),
		funcs:    funcSpans(fset, file),
		declared: declaredNames(file),
		builtins: builtinCalls(fset, file),
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			text, ok := markText(c.Text)
			if !ok {
				continue
			}
			at := fset.Position(c.Slash)
			line := at.Line
			if strings.TrimSpace(t.lines[line-1][:at.Column-1]) != "" {
				return nil, fmt.Errorf("%s:%d: an output line stands alone on its line, and code precedes this one", filename, line)
			}
			if t.blockOf(line) == (span{}) {
				return nil, fmt.Errorf("%s:%d: an output line belongs inside a function body", filename, line)
			}
			t.outputs[line] = text
		}
	}
	return t, nil
}

// builtin is one call to a built-in, with its 1-based position.
type builtin struct {
	name     string
	line     int
	from, to int
}

// builtinsOn returns the built-in calls on one scaffolding line.
func (t *template) builtinsOn(line int) []builtin {
	var calls []builtin
	for _, b := range t.builtins {
		if b.line == line {
			calls = append(calls, b)
		}
	}
	return calls
}

// gensym is the built-in that hands out a name of the generated program.
const gensym = "GENSYM"

// builtinCalls finds every call to a built-in in the scaffolding.
func builtinCalls(fset *token.FileSet, file *ast.File) []builtin {
	var calls []builtin
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != gensym {
			return true
		}
		at := fset.Position(id.Pos())
		calls = append(calls, builtin{name: id.Name, line: at.Line, from: at.Column, to: at.Column + len(id.Name)})
		return true
	})
	return calls
}

// span is the line range of a block, including both ends.
type span struct{ from, to int }

// blockOf returns the innermost block that holds a line, or the zero span.
func (t *template) blockOf(line int) span {
	var found span
	for _, b := range t.blocks {
		if b.from > line || line > b.to {
			continue
		}
		if found == (span{}) || b.from >= found.from && b.to <= found.to {
			found = b
		}
	}
	return found
}

// funcOf returns the innermost function body that holds a line, or the zero span.
func (t *template) funcOf(line int) span {
	var found span
	for _, f := range t.funcs {
		if f.from > line || line > f.to {
			continue
		}
		if found == (span{}) || f.from >= found.from && f.to <= found.to {
			found = f
		}
	}
	return found
}

// funcSpans returns the line range of every function body in the file.
func funcSpans(fset *token.FileSet, file *ast.File) []span {
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

// blockSpans returns the line range of every block and case clause in the file.
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

// declaredNames collects every name with a binding in the scaffolding.
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
