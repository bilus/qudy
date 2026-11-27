// Package gen checks whether rapid can generate well-typed Go programs
// that are comptime by construction, and plant one known impurity in them.
package gen

import (
	"fmt"
	"sort"
	"strings"

	"pgregory.net/rapid"
)

// typ is one of the three types the generated fragment computes with.
type typ int

const (
	tInt typ = iota
	tBool
	tString
)

// String returns the Go spelling of the type. The fragment uses int64, never int.
func (t typ) String() string { return [...]string{"int64", "bool", "string"}[t] }

// sig is the signature of a generated function.
type sig struct {
	name   string
	params []typ
	result typ
}

// Program is a generated program and what is known about it by construction.
type Program struct {
	Src      string
	Funcs    []string // every generated function, in declaration order
	Impurity string   // the planted impurity, or "" for a pure program
	Layers   []string // the indirections between the victim and the impurity
	Victim   string   // the function whose result depends on the impurity
	Tainted  []string // the victim and every function with a call path to it
}

// builder generates one program from rapid's draws.
type builder struct {
	t     *rapid.T
	funcs []sig
	read  map[typ][]string // variables an expression may read
	write map[typ][]string // variables a statement may assign
	n     int
	calls map[string]map[string]bool // caller to callees
	cur   string
}

// draw picks an index below n; index 0 is the simplest alternative, so shrinking moves toward it.
func (b *builder) draw(n int, label string) int {
	return rapid.IntRange(0, n-1).Draw(b.t, label)
}

// fresh returns a new local name.
func (b *builder) fresh(prefix string) string {
	b.n++
	return fmt.Sprintf("%s%d", prefix, b.n)
}

// literal returns a literal of the type.
func (b *builder) literal(want typ) string {
	switch want {
	case tInt:
		return fmt.Sprintf("int64(%d)", rapid.IntRange(-3, 9).Draw(b.t, "int"))
	case tBool:
		return fmt.Sprint(rapid.Bool().Draw(b.t, "bool"))
	}
	return fmt.Sprintf("%q", rapid.SampledFrom([]string{"", "a", "go", "é"}).Draw(b.t, "string"))
}

// expr returns an expression of the wanted type that reads only variables in scope.
func (b *builder) expr(want typ, depth int) string {
	alts := []func() string{func() string { return b.literal(want) }}
	if vars := b.read[want]; len(vars) > 0 {
		alts = append(alts, func() string { return rapid.SampledFrom(vars).Draw(b.t, "var") })
	}
	binary := func(op string, operand typ) func() string {
		return func() string {
			return "(" + b.expr(operand, depth-1) + " " + op + " " + b.expr(operand, depth-1) + ")"
		}
	}
	if depth > 0 {
		switch want {
		case tInt:
			alts = append(alts, binary("+", tInt), binary("-", tInt), binary("*", tInt))
			alts = append(alts, func() string { return "int64(len(" + b.expr(tString, depth-1) + "))" })
		case tBool:
			alts = append(alts, binary("<", tInt), binary("==", tInt), binary("==", tString), binary("&&", tBool))
			alts = append(alts, func() string { return "!" + b.expr(tBool, depth-1) })
		case tString:
			alts = append(alts, func() string { return "clip" + binary("+", tString)() })
			alts = append(alts, func() string { return "digits(" + b.expr(tInt, depth-1) + ")" })
		}
		for _, f := range b.funcs {
			if f.result == want {
				alts = append(alts, func() string { return b.call(f, depth-1) })
			}
		}
	}
	return alts[b.draw(len(alts), "alt")]()
}

// call returns a call of an earlier function and records the edge of the call graph.
func (b *builder) call(f sig, depth int) string {
	args := make([]string, len(f.params))
	for i, p := range f.params {
		args[i] = b.expr(p, depth)
	}
	if b.cur != "" {
		b.calls[b.cur][f.name] = true
	}
	return f.name + "(" + strings.Join(args, ", ") + ")"
}

// scoped runs gen and then drops the variables it declared.
func (b *builder) scoped(gen func() string) string {
	marks := [2][3]int{}
	for t := tInt; t <= tString; t++ {
		marks[0][t], marks[1][t] = len(b.read[t]), len(b.write[t])
	}
	out := gen()
	for t := tInt; t <= tString; t++ {
		b.read[t], b.write[t] = b.read[t][:marks[0][t]], b.write[t][:marks[1][t]]
	}
	return out
}

// stmts returns up to three statements. Loops have literal bounds, so every program terminates.
func (b *builder) stmts(depth int, indent string) string {
	var out strings.Builder
	for range b.draw(4, "stmts") {
		kinds := 2
		if depth > 0 {
			kinds = 4
		}
		switch b.draw(kinds, "stmt") {
		case 0:
			t := typ(b.draw(3, "type"))
			name, value := b.fresh("v"), b.expr(t, 2)
			fmt.Fprintf(&out, "%s%s := %s\n%s_ = %s\n", indent, name, value, indent, name)
			b.read[t], b.write[t] = append(b.read[t], name), append(b.write[t], name)
		case 1:
			t := typ(b.draw(3, "type"))
			if len(b.write[t]) == 0 {
				continue
			}
			name := rapid.SampledFrom(b.write[t]).Draw(b.t, "target")
			fmt.Fprintf(&out, "%s%s = %s\n", indent, name, b.expr(t, 2))
		case 2:
			cond := b.expr(tBool, 2)
			then := b.scoped(func() string { return b.stmts(depth-1, indent+"\t") })
			els := b.scoped(func() string { return b.stmts(depth-1, indent+"\t") })
			fmt.Fprintf(&out, "%sif %s {\n%s%s} else {\n%s%s}\n", indent, cond, then, indent, els, indent)
		case 3:
			i, bound := b.fresh("i"), b.draw(4, "bound")
			body := b.scoped(func() string {
				b.read[tInt] = append(b.read[tInt], i) // readable, never assignable
				return b.stmts(depth-1, indent+"\t")
			})
			fmt.Fprintf(&out, "%sfor %s := int64(0); %s < %d; %s++ {\n%s%s}\n", indent, i, i, bound, i, body, indent)
		}
	}
	return out.String()
}

// function returns the declaration of the next function; mix, if set, wraps its result.
func (b *builder) function(i int, mix func(t typ, e string) string) string {
	s := sig{name: fmt.Sprintf("F%d", i), result: typ(b.draw(3, "result"))}
	b.cur, b.calls[s.name] = s.name, map[string]bool{}
	b.read, b.write = map[typ][]string{}, map[typ][]string{}
	var params []string
	for j := range b.draw(4, "params") {
		t, name := typ(b.draw(3, "type")), fmt.Sprintf("p%d", j)
		s.params = append(s.params, t)
		params = append(params, name+" "+t.String())
		b.read[t], b.write[t] = append(b.read[t], name), append(b.write[t], name)
	}
	body := b.stmts(2, "\t")
	result := b.expr(s.result, 2)
	if mix != nil {
		result = mix(s.result, result)
	}
	b.funcs, b.cur = append(b.funcs, s), ""
	return fmt.Sprintf("func %s(%s) %s {\n%s\treturn %s\n}\n\n", s.name, strings.Join(params, ", "), s.result, body, result)
}

// helpers are comptime by inspection, and keep generated strings short.
const helpers = `func clip(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

func digits(n int64) string {
	if n == 0 {
		return "0"
	}
	neg, out := n < 0, ""
	for n != 0 {
		d := n % 10
		if d < 0 {
			d = -d
		}
		out = string(rune('0'+d)) + out
		n /= 10
	}
	if neg {
		return "-" + out
	}
	return out
}

`

// Generate draws a program. With plant set, one function's result depends on one impurity.
func Generate(t *rapid.T, plant bool) Program {
	b := &builder{t: t, calls: map[string]map[string]bool{}}
	p := Program{}
	n := 1 + b.draw(4, "funcs")
	victim := -1
	var imp impurity
	var hidden, hiddenDecls string
	imports := map[string]bool{"fmt": true}
	if plant {
		victim = b.draw(n, "victim")
		imp = impurities[b.draw(len(impurities), "impurity")]
		for _, path := range imp.imports {
			imports[path] = true
		}
		hidden = "impure"
		hiddenDecls = imp.decls + "func impure() int64 {\n\t" + imp.body + "\n}\n\n"
		for i := range b.draw(4, "depth") {
			l := layers[b.draw(len(layers), "layer")]
			decls, name := l.wrap(i, hidden)
			for _, path := range l.imports {
				imports[path] = true
			}
			hiddenDecls, hidden = hiddenDecls+decls, name
			p.Layers = append(p.Layers, l.name)
		}
		p.Impurity, p.Victim = imp.name, fmt.Sprintf("F%d", victim)
	}
	var decls strings.Builder
	for i := range n {
		var mix func(typ, string) string
		if i == victim {
			mix = func(t typ, e string) string {
				switch t {
				case tInt:
					return e + " + " + hidden + "()"
				case tBool:
					return e + " != (" + hidden + "()%2 == 0)"
				}
				return "clip(" + e + " + digits(" + hidden + "()))"
			}
		}
		decls.WriteString(b.function(i, mix))
	}
	// round is a new function: no parameter of the last generated function is in scope.
	b.read, b.write = map[typ][]string{}, map[typ][]string{}
	var round strings.Builder
	for _, f := range b.funcs {
		p.Funcs = append(p.Funcs, f.name)
		fmt.Fprintf(&round, "\tfmt.Println(%q, %s)\n", f.name, b.call(f, 0))
	}
	p.Tainted = tainted(b.calls, p.Victim)
	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, fmt.Sprintf("\t%q\n", path))
	}
	sort.Strings(paths)
	p.Src = "package main\n\nimport (\n" + strings.Join(paths, "") + ")\n\n" + helpers + hiddenDecls + decls.String() +
		"func round() {\n" + round.String() + "}\n\nfunc main() {\n\tround()\n\tfmt.Println(\"--\")\n\t" + imp.between + "\n\tround()\n}\n"
	return p
}

// tainted returns the victim and every function that can reach it in the call graph.
func tainted(calls map[string]map[string]bool, victim string) []string {
	if victim == "" {
		return nil
	}
	set := map[string]bool{victim: true}
	for changed := true; changed; {
		changed = false
		for caller, callees := range calls {
			for callee := range callees {
				if set[callee] && !set[caller] {
					set[caller], changed = true, true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
