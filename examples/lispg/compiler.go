//go:build qudy || generate

// Command lispg emits Go directly while walking Lisp reader forms.
package main

//go:generate go run ../../cmd/qudy -emit emit -o compiler_gen.go compiler.go

import (
	"fmt"
	"go/token"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
)

// A reader form retains only delimiters, atoms, metadata and source position.
// There are no nodes for calls, bindings, functions or other language constructs.
type form struct {
	atom  string
	delim rune
	kids  []form
	hint  string
	pos   scanner.Position
}

func fail(f form, message string) { panic(fmt.Errorf("%s: %s", f.pos, message)) }

func need(f form, ok bool, message string) {
	if !ok {
		fail(f, message)
	}
}

func read(s *scanner.Scanner, t rune) form {
	f := form{atom: s.TokenText(), pos: s.Position}
	if t == '^' {
		hint := read(s, s.Scan())
		need(hint, hint.delim == 0, "expected a type after ^")
		f = read(s, s.Scan())
		f.hint = hint.atom
		return f
	}
	need(f, t != scanner.EOF && t != ')' && t != ']', "unexpected end or closing delimiter")
	if t != '(' && t != '[' {
		return f
	}
	f.delim, f.atom = t, ""
	close := ')'
	if t == '[' {
		close = ']'
	}
	for t = s.Scan(); t != close; t = s.Scan() {
		if t == ';' {
			for s.Peek() != '\n' && s.Peek() != scanner.EOF {
				s.Next()
			}
			continue
		}
		f.kids = append(f.kids, read(s, t))
	}
	return f
}

func ident(f form) string {
	need(f, f.delim == 0 && f.atom != "_" && token.IsIdentifier(f.atom), "expected a Go identifier")
	return f.atom
}

func typ(f form, t string) string {
	need(f, t == "int64" || t == "float64" || t == "string" || t == "bool", "type must be int64, float64, string or bool")
	return t
}

func parts(f form) (string, []form) {
	need(f, f.delim == '(' && len(f.kids) > 0, "expected a nonempty list")
	return f.kids[0].atom, f.kids[1:]
}

func arity(f form, args []form, min, max int) {
	need(f, len(args) >= min && len(args) <= max, "wrong number of arguments")
}

// emit is Qudy's printf target. A write error must also fail the transpiler.
func emit(format string, args ...any) {
	if _, err := fmt.Printf(format, args...); err != nil {
		panic(err)
	}
}

func atom(f form) {
	s := f.atom
	need(f, s != "", "expected an atom")
	if strings.HasPrefix(s, "\"") {
		_, err := strconv.Unquote(s)
		need(f, err == nil, "invalid string")
		//`~s\
	} else if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		//`int64(~n)\
	} else if strings.ContainsAny(s, ".eE") && numeric(s) {
		_, err := strconv.ParseFloat(s, 64)
		need(f, err == nil, "invalid float64 literal")
		//`float64(~s)\
	} else {
		for _, p := range strings.Split(s, ".") {
			ident(form{atom: p, pos: f.pos})
		}
		//`~s\
	}
}

func numeric(s string) bool {
	s = strings.TrimLeft(s, "+-")
	return len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '.')
}

// A known result type flows down from a function return or a ^type hint.
// Go checks ordinary calls, names and operand types in the emitted program.
func expr(f form, want string) {
	if f.hint != "" {
		want = typ(f, f.hint)
	}
	if f.delim == 0 {
		atom(f)
		return
	}
	head, args := parts(f)
	switch head {
	case "if", "when", "let":
		need(f, want != "", "expression needs a ^type hint or a typed function result")
		//`func() ~want {
		localExpr(f, want)
		//`}()\
	case "fn":
		arity(f, args, 2, len(args))
		signature(args[0], args[0].hint)
		body(args[1:], args[0].hint)
		//`}\
	case "defn", "package", "import":
		fail(f, head+" can only be used at top level")
	case "+", "-", "*", "/", "%", "=", "not=", "<", "<=", ">", ">=", "and", "or", "not":
		operator(f, head, args, want)
	default:
		expr(f.kids[0], "")
		//`(\
		for i, a := range args {
			if i > 0 {
				//`, \
			}
			expr(a, "")
		}
		//`)\
	}
}

func operator(f form, op string, args []form, want string) {
	min, max := 2, len(args)
	if op == "-" {
		min = 1
	}
	if strings.Contains(" = not= < <= > >= % ", " "+op+" ") {
		max = 2
		want = ""
	}
	if op == "and" || op == "or" || op == "not" {
		want = "bool"
	}
	if op == "not" {
		min, max = 1, 1
	}
	arity(f, args, min, max)
	if goOp, ok := map[string]string{"=": "==", "not=": "!=", "and": "&&", "or": "||", "not": "!"}[op]; ok {
		op = goOp
	}
	//`(\
	if len(args) == 1 {
		//`~op\
	}
	for i, a := range args {
		if i > 0 {
			//` ~op \
		}
		expr(a, want)
	}
	//`)\
}

func signature(params form, result string) {
	//`func\
	parameters(params, result)
}

func parameters(params form, result string) {
	need(params, params.delim == '[', "expected parameter vector")
	if result != "" {
		typ(params, result)
	}
	//`(\
	seen := map[string]bool{}
	for i, p := range params.kids {
		name, t := ident(p), typ(p, p.hint)
		need(p, !seen[name], "duplicate parameter")
		seen[name] = true
		if i > 0 {
			//`, \
		}
		//`~name ~t\
	}
	//`) ~result {
}

func body(forms []form, result string) {
	if len(forms) == 0 {
		zero(result)
	}
	for i, f := range forms {
		if i == len(forms)-1 && result != "" {
			//`return \
			expr(f, result)
			//`
		} else {
			discardExpr(f)
		}
	}
}

// An absent branch or empty body yields the result type's zero value.
func zero(result string) {
	if result != "" {
		value := map[string]string{"int64": "0", "float64": "0", "string": "\"\"", "bool": "false"}[result]
		//`return ~value
	}
}

func conditional(args []form, result string) {
	//`if \
	expr(args[0], "bool")
	//` {
	body(args[1:2], result)
	if len(args) == 3 {
		//`} else {
		body(args[2:3], result)
	}
	//`}
	if len(args) == 2 {
		zero(result)
	}
}

func binding(f form, args []form, result string) {
	arity(f, args, 1, len(args))
	v := args[0]
	need(v, v.delim == '[' && len(v.kids)%2 == 0, "expected name/value binding pairs")
	// Each nested scope lets an initializer see the previous binding of its name.
	for i := 0; i < len(v.kids); i += 2 {
		name := ident(v.kids[i])
		//`{
		t := v.kids[i].hint
		if t != "" {
			typ(v.kids[i], t)
			//`var ~name ~t = \
		} else {
			//`~name := \
		}
		expr(v.kids[i+1], t)
		//`
		//`_ = ~name
	}
	body(args[1:], result)
	for i := 0; i < len(v.kids); i += 2 {
		//`}
	}
}

// Emit the same expressions without materializing a result when it is discarded.
func localExpr(f form, result string) {
	head, args := parts(f)
	switch head {
	case "if":
		arity(f, args, 2, 3)
		conditional(args, result)
	case "when":
		arity(f, args, 1, len(args))
		//`if \
		expr(args[0], "bool")
		//` {
		body(args[1:], result)
		//`}
		zero(result)
	case "let":
		//`{
		binding(f, args, result)
		//`}
	}
}

func discardExpr(f form) {
	if f.delim == '(' {
		head, _ := parts(f)
		switch head {
		case "if", "when", "let":
			localExpr(f, "")
			return
		}
		// Calls may return zero, one, or several values; Go discards them here.
		if head != "fn" && !strings.Contains(" + - * / % = not= < <= > >= and or not ", " "+head+" ") {
			expr(f, "")
			//`
			return
		}
	}
	//`_ = \
	expr(f, "")
	//`
}

func namespaceExpr(f form, stage *int) {
	head, args := parts(f)
	switch head {
	case "package":
		arity(f, args, 1, 1)
		need(f, *stage == 0, "package must appear once, first")
		name := ident(args[0])
		//`package ~name
		*stage = 1
	case "import":
		need(f, *stage == 1, "imports must follow package and precede functions")
		for _, a := range args {
			path, err := strconv.Unquote(a.atom)
			need(a, a.delim == 0 && err == nil && path != "", "expected quoted import path")
			//`import ~"path
		}
	case "defn":
		arity(f, args, 3, len(args))
		need(f, *stage > 0, "package must come first")
		*stage = 2
		name, result := ident(args[0]), args[0].hint
		if result != "" {
			typ(args[0], result)
		}
		//`func ~name\
		parameters(args[1], result)
		body(args[2:], result)
		//`}
	default:
		fail(f, "expected package, import or defn")
	}
}

func main() {
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintln(os.Stderr, "lispg:", e)
			os.Exit(1)
		}
	}()
	in := os.Stdin
	if len(os.Args) > 2 {
		panic(fmt.Errorf("usage: lispg [source.lisp]"))
	}
	if len(os.Args) == 2 {
		var err error
		in, err = os.Open(os.Args[1])
		if err != nil {
			panic(err)
		}
		defer in.Close()
	}
	var s scanner.Scanner
	s.Init(in)
	s.Filename = in.Name()
	s.Mode = scanner.ScanIdents | scanner.ScanStrings
	s.Whitespace |= 1 << ','
	s.IsIdentRune = func(r rune, i int) bool {
		return r != scanner.EOF && !unicode.IsSpace(r) && !strings.ContainsRune("()[]^;\",", r)
	}
	s.Error = func(s *scanner.Scanner, msg string) { panic(fmt.Errorf("%s: %s", s.Position, msg)) }
	stage := 0
	for t := s.Scan(); t != scanner.EOF; t = s.Scan() {
		if t == ';' {
			for s.Peek() != '\n' && s.Peek() != scanner.EOF {
				s.Next()
			}
			continue
		}
		namespaceExpr(read(&s, t), &stage)
	}
	if stage == 0 {
		panic(fmt.Errorf("missing package form"))
	}
}
