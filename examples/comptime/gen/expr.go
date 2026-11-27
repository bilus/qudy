package gen

import (
	"fmt"
	"strings"

	"pgregory.net/rapid"
)

// ifaceCost bounds the cost of a method, and of a function used as a value.
// litCost bounds the cost of a call inside a function literal.
const (
	ifaceCost = 1000
	litCost   = 50
)

// strs are the string literals of generated programs. None holds a line break.
var strs = []string{"", "a", "go", "é", "b c", "Zed"}

// safe reports whether the statement being written must not gain an operation that may panic.
// A statement holds at most one such operation, because Go leaves the order of some evaluations open.
func (m *machine) safe() bool { return m.fn.calm || m.risky }

// panics records that the statement being written may panic.
func (m *machine) panics(why string) {
	m.risky, m.fn.panics = true, true
	m.feature("may panic: " + why)
}

// find returns the indices of the variables in scope that pass the test.
func (m *machine) find(keep func(variable) bool) []int {
	var out []int
	for i, v := range m.vars {
		if keep(v) {
			out = append(out, i)
		}
	}
	return out
}

// read returns a variable as a value. A slice read as a whole may gain an alias.
func (m *machine) read(i int) string {
	if m.vars[i].typ.kind == kSlice {
		m.vars[i].unique = false
	}
	return m.vars[i].name
}

// projected estimates the cost of one call if the next statement adds extra operations.
func (m *machine) projected(extra float64) float64 {
	c := m.cost + extra
	for i := len(m.blocks) - 1; i >= 0; i-- {
		c = m.blocks[i].outer + m.blocks[i].mult*c
	}
	return c
}

// room reports whether the open function can afford extra operations here.
func (m *machine) room(extra float64) bool { return m.projected(extra) <= m.fn.limit }

// cur returns the package whose source is being written.
func (m *machine) cur() *pkg {
	if m.fn != nil {
		return m.fn.pkg
	}
	return m.pkg()
}

// qualify returns a name of a package as the current package spells it.
func (m *machine) qualify(p *pkg, name string) string {
	if p == m.cur() {
		return name
	}
	m.cur().imports[p.path] = true
	if m.fn != nil {
		m.feature("use of another package")
	}
	return p.name + "." + name
}

// tname returns the spelling of a type in the current package.
func (m *machine) tname(t *typ) string {
	switch t.kind {
	case kInt:
		return "int64"
	case kBool:
		return "bool"
	case kString:
		return "string"
	case kSlice:
		return "[]" + m.tname(t.elem)
	case kMap:
		return "map[string]int64"
	case kPointer:
		return "*" + m.qualify(t.decl.pkg, t.decl.name)
	case kFunc:
		return "func(int64) int64"
	case kParam:
		return "T"
	case kPick:
		return "func(T) int64"
	}
	return m.qualify(t.decl.pkg, t.decl.name)
}

// literal returns a literal of a basic type.
func (m *machine) literal(t *rapid.T, want *typ) string {
	switch want {
	case tInt:
		return fmt.Sprintf("int64(%d)", rapid.IntRange(-3, 9).Draw(t, "int"))
	case tBool:
		return fmt.Sprint(rapid.Bool().Draw(t, "bool"))
	}
	return fmt.Sprintf("%q", strs[pick(t, len(strs), "string")])
}

// construct returns an expression that creates a new value of the type.
func (m *machine) construct(t *rapid.T, want *typ) string {
	switch want.kind {
	case kSlice:
		elems := make([]string, pick(t, 4, "elements"))
		for i := range elems {
			elems[i] = m.expr(t, want.elem, 1)
		}
		m.feature("slice literal")
		return m.tname(want) + "{" + strings.Join(elems, ", ") + "}"
	case kMap:
		entries := make([]string, pick(t, 3, "entries"))
		for i := range entries {
			entries[i] = fmt.Sprintf("%q: %s", fmt.Sprintf("k%d", i), m.expr(t, tInt, 1))
		}
		m.feature("map literal")
		return "map[string]int64{" + strings.Join(entries, ", ") + "}"
	case kStruct, kPointer:
		var fields []string
		for _, f := range want.decl.fields {
			if rapid.Bool().Draw(t, "set field") {
				fields = append(fields, f.name+": "+m.expr(t, f.typ, 1))
			}
		}
		m.feature("struct literal")
		// The parentheses keep the literal legal in the header of an if statement.
		lit := m.qualify(want.decl.pkg, want.decl.name) + "{" + strings.Join(fields, ", ") + "}"
		if want.kind == kPointer {
			return "(&" + lit + ")"
		}
		return "(" + lit + ")"
	case kFunc:
		return m.funcLit(t, tInt)
	case kIface:
		return m.qualify(want.decl.pkg, want.decl.name) + "(nil)"
	case kParam:
		for _, v := range m.vars {
			if v.typ == tParam {
				return v.name
			}
		}
	}
	return m.literal(t, want)
}

// funcLit returns a function literal that takes a value of the type and cannot panic.
func (m *machine) funcLit(t *rapid.T, param *typ) string {
	x := fmt.Sprintf("x%d", m.next())
	risky, lit := m.risky, m.lit
	m.risky, m.lit = true, true
	m.vars = append(m.vars, variable{name: x, typ: param})
	body := m.expr(t, tInt, 1)
	m.vars = m.vars[:len(m.vars)-1]
	m.risky, m.lit = risky, lit
	m.feature("function literal")
	return "func(" + x + " " + m.tname(param) + ") int64 { return " + body + " }"
}

// usable reports whether the open function may refer to a finished function.
func (m *machine) usable(g *fn) bool {
	return (g.exported || g.pkg == m.cur()) && (m.fn.loose || !g.reach)
}

// each calls do for every finished function the open function may refer to.
// The partner of a recursive function is left out: only the final call may reach it.
func (m *machine) each(do func(g *fn)) {
	for _, p := range m.pkgs {
		for _, g := range p.funcs {
			if m.usable(g) && g != m.fn.partner {
				do(g)
			}
		}
	}
}

// callable reports whether a call of the function fits the statement being written.
func (m *machine) callable(g *fn) bool {
	if m.lit && g.cost > litCost {
		return false
	}
	return (!g.panics || !m.safe()) && m.room(g.cost)
}

// refer records that the open function refers to a function, and returns its name here.
func (m *machine) refer(g *fn) string {
	m.fn.refs = append(m.fn.refs, g)
	return m.qualify(g.pkg, g.name)
}

// call returns a call of a finished function.
func (m *machine) call(t *rapid.T, g *fn, depth int) string {
	if g.panics {
		m.panics("call of a function that may panic")
	}
	m.cost += g.cost
	var name string
	if g.recv != nil {
		recv := g.recv.typ
		if g.ptrRecv {
			recv = g.recv.ptr
		}
		m.fn.refs = append(m.fn.refs, g)
		name = m.expr(t, recv, depth) + "." + g.name
		m.feature("method call")
	} else {
		name = m.refer(g)
		m.feature("call")
	}
	var args []string
	inst := tInt
	if g.kind == "generic" {
		inst = basics[pick(t, len(basics), "type argument")]
		name += "[" + m.tname(inst) + "]"
		m.feature("call of a generic function")
	}
	for i, p := range g.params {
		switch {
		case i == 0 && g.kind == "recursive" && m.fn.kind == "main":
			// The generated main function makes the recursion run.
			args = append(args, fmt.Sprintf("int64(%d)", 1+pick(t, 3, "depth")))
			continue
		}
		switch p.typ {
		case tParam:
			args = append(args, m.expr(t, inst, depth))
		case tPick:
			args = append(args, m.funcLit(t, inst))
		default:
			args = append(args, m.expr(t, p.typ, depth))
		}
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}

// expr returns an expression of the type. Index zero of every choice is the simplest.
func (m *machine) expr(t *rapid.T, want *typ, depth int) string {
	alts := []func() string{func() string { return m.construct(t, want) }}
	add := func(f func() string) { alts = append(alts, f) }
	sub := func(want *typ) string { return m.expr(t, want, depth-1) }
	binary := func(op string, operand *typ) {
		add(func() string { return "(" + sub(operand) + " " + op + " " + sub(operand) + ")" })
	}
	for _, i := range m.find(func(v variable) bool { return v.typ == want }) {
		add(func() string { return m.read(i) })
	}
	if want.kind == kPointer {
		add(func() string { return "(" + m.tname(want) + ")(nil)" })
	}
	if depth <= 0 {
		return alts[pick(t, len(alts), "alt")]()
	}
	switch want {
	case tInt:
		binary("+", tInt)
		binary("-", tInt)
		binary("*", tInt)
		binary("&", tInt)
		add(func() string { return fmt.Sprintf("(%s / %d)", sub(tInt), 1+pick(t, 7, "divisor")) })
		add(func() string { return fmt.Sprintf("(%s %% %d)", sub(tInt), 1+pick(t, 7, "divisor")) })
		add(func() string { return fmt.Sprintf("(%s << %d)", sub(tInt), pick(t, 9, "shift")) })
		add(func() string { return "int64(len(" + sub(tString) + "))" })
		for _, i := range m.find(func(v variable) bool { return v.typ.kind == kSlice || v.typ.kind == kMap }) {
			add(func() string { return "int64(len(" + m.vars[i].name + "))" })
		}
		for _, i := range m.find(func(v variable) bool { return v.typ.kind == kMap }) {
			add(func() string {
				m.feature("map read")
				return m.vars[i].name + "[" + sub(tString) + "]"
			})
		}
		for _, i := range m.find(func(v variable) bool { return v.typ == tFunc }) {
			if m.room(ifaceCost) && !m.lit {
				add(func() string {
					m.cost += ifaceCost
					m.feature("call of a function value")
					return m.vars[i].name + "(" + sub(tInt) + ")"
				})
			}
		}
		for _, i := range m.find(func(v variable) bool { return v.typ == tPick }) {
			add(func() string {
				m.cost += litCost
				m.feature("call of a function value")
				return m.vars[i].name + "(" + m.construct(t, tParam) + ")"
			})
		}
	case tBool:
		add(func() string { return "!" + sub(tBool) })
		binary("<", tInt)
		binary("==", tInt)
		binary("==", tString)
		binary("<", tString)
		binary("&&", tBool)
		binary("||", tBool)
	case tString:
		add(func() string { return m.clip(sub(tString) + " + " + sub(tString)) })
	}
	m.fields(want, add)
	m.externals(t, want, depth, add)
	m.each(func(g *fn) {
		if g.result == want && m.callable(g) {
			add(func() string { return m.call(t, g, depth-1) })
		}
	})
	if want == tFunc {
		m.funcValues(t, depth, add)
	}
	if want.kind == kIface {
		m.conversions(t, want, depth, add)
	}
	if !m.safe() {
		m.risks(t, want, depth, add)
	}
	return alts[pick(t, len(alts), "alt")]()
}

// fields adds the reads of fields of the type that cannot panic.
func (m *machine) fields(want *typ, add func(func() string)) {
	if !want.basic() {
		return
	}
	for _, i := range m.find(func(v variable) bool { return v.typ.kind == kStruct || v.typ.kind == kPointer && v.owned }) {
		for _, f := range m.vars[i].typ.decl.fields {
			if f.typ == want {
				add(func() string {
					m.feature("field read")
					return m.vars[i].name + "." + f.name
				})
			}
		}
	}
}

// externals adds the calls of trusted symbols that return the type.
func (m *machine) externals(t *rapid.T, want *typ, depth int, add func(func() string)) {
	if !m.cfg.Constructs.Trusted {
		return
	}
	ints := m.find(func(v variable) bool { return v.typ == tInt })
	for _, e := range index {
		if e.result != want || e.nonconst && len(ints) == 0 {
			continue
		}
		add(func() string {
			args := make([]any, len(e.params))
			for i, p := range e.params {
				if e.nonconst && p == tInt {
					args[i] = m.vars[ints[pick(t, len(ints), "variable")]].name
					continue
				}
				args[i] = m.expr(t, p, depth-1)
			}
			m.cur().imports[e.path] = true
			m.feature("trusted symbol")
			call := fmt.Sprintf(e.call, args...)
			if want == tString {
				return m.clip(call)
			}
			return call
		})
	}
}

// funcValues adds the functions and methods that can be used as a func(int64) int64.
func (m *machine) funcValues(t *rapid.T, depth int, add func(func() string)) {
	m.each(func(g *fn) {
		if g.result != tInt || len(g.params) != 1 || g.params[0].typ != tInt || g.panics || g.cost > ifaceCost {
			return
		}
		if g.recv == nil {
			add(func() string {
				m.feature("function value")
				return m.refer(g)
			})
			return
		}
		add(func() string {
			m.feature("method value")
			m.fn.refs = append(m.fn.refs, g)
			recv := g.recv.typ
			if g.ptrRecv {
				recv = g.recv.ptr
			}
			return m.construct(t, recv) + "." + g.name
		})
	})
}

// conversions adds the conversions of struct values to the interface type.
func (m *machine) conversions(t *rapid.T, want *typ, depth int, add func(func() string)) {
	for _, g := range want.decl.impls {
		if !m.fn.loose && g.reach {
			continue
		}
		add(func() string {
			m.fn.refs = append(m.fn.refs, g)
			m.feature("conversion to an interface type")
			from := g.recv.typ
			if g.ptrRecv {
				from = g.recv.ptr
			}
			return m.tname(want) + "(" + m.expr(t, from, depth-1) + ")"
		})
	}
}

// risks adds the expressions of the type that may panic.
func (m *machine) risks(t *rapid.T, want *typ, depth int, add func(func() string)) {
	if want == tInt {
		for _, i := range m.find(func(v variable) bool { return v.typ == tInt }) {
			add(func() string {
				m.panics("division")
				return "(" + m.expr(t, tInt, depth-1) + " / " + m.vars[i].name + ")"
			})
		}
		for _, i := range m.find(func(v variable) bool { return v.typ == tString }) {
			add(func() string {
				m.panics("index")
				return fmt.Sprintf("int64(%s[%d])", m.vars[i].name, pick(t, 4, "index"))
			})
		}
		for _, i := range m.find(func(v variable) bool { return v.typ.kind == kIface }) {
			if m.room(ifaceCost) {
				add(func() string {
					m.panics("interface method call")
					m.cost += ifaceCost
					m.feature("interface method call")
					arg := m.expr(t, tInt, depth-1)
					return m.vars[i].name + "." + m.vars[i].typ.decl.method + "(" + arg + ")"
				})
			}
		}
	}
	if !want.basic() {
		return
	}
	for _, i := range m.find(func(v variable) bool { return v.typ.kind == kSlice && v.typ.elem == want }) {
		add(func() string {
			m.panics("index")
			return fmt.Sprintf("%s[%d]", m.vars[i].name, pick(t, 4, "index"))
		})
	}
	for _, i := range m.find(func(v variable) bool { return v.typ.kind == kPointer && !v.owned }) {
		for _, f := range m.vars[i].typ.decl.fields {
			if f.typ == want {
				add(func() string {
					m.panics("nil pointer")
					m.feature("field read")
					return m.vars[i].name + "." + f.name
				})
			}
		}
	}
}
