// Package gen generates Go programs for testing a comptime analyzer.
//
// A program is built by a state machine whose actions open packages, declare
// types, open functions and add statements to the open function. Every program
// type-checks and terminates. The generator labels each function it writes:
// a function is Comptime when it is built only from the comptime fragment, the
// trusted symbols of the index, and other Comptime functions.
package gen

import (
	"fmt"
	"strings"

	"pgregory.net/rapid"
)

// Config bounds the size and the contents of a generated program.
type Config struct {
	// Rounds is the number of rounds of actions. Each round runs as many
	// actions as the flag -rapid.steps asks for, 30 on average by default.
	Rounds int
	// Packages is the largest number of packages besides main.
	Packages int
	// Impurities is the largest number of planted impurities. With zero, every
	// function is Comptime; otherwise at least one function is Impure.
	Impurities int
}

// Default generates programs of a few hundred lines with up to three impurities.
var Default = Config{Rounds: 6, Packages: 3, Impurities: 3}

// Pure generates programs of the same size in which every function is Comptime.
var Pure = Config{Rounds: 6, Packages: 3}

// Limits that keep a generated program small and quick to run.
const (
	maxBlocks  = 3     // nested blocks in a function body
	maxLoops   = 2     // nested loops
	maxCost    = 20000 // estimated operations in one call
	exprDepth  = 3
	rangeLimit = 8  // iterations of a loop over a slice or a string
	clipLen    = 24 // bytes of the longest string
)

// between is an assignment to a package variable that main executes between its two rounds.
type between struct {
	pkg  *pkg
	stmt string
}

// block is an open if statement or loop of the open function.
type block struct {
	kind  string // "if", "else" or "loop"
	vars  int    // the number of variables in scope when it opened
	mult  float64
	outer float64 // the cost of the enclosing block so far
}

// machine is the state of the generator: the program so far and the open function.
type machine struct {
	cfg     Config
	module  string
	pkgs    []*pkg
	fn      *fn
	body    strings.Builder
	vars    []variable
	blocks  []block
	cost    float64
	risky   bool // the statement being written already holds an operation that may panic
	lit     bool // a function literal is being written
	only    int  // the index of the one impurity to plant, or -1 for any
	n       int
	planted int
	between []between // the assignments main executes between its two rounds
}

// Generate draws a program. The same draws give the same program.
//
// The number of actions in a round comes from rapid, so a program also depends
// on the flags -rapid.steps and -short. Hold them fixed to get the same program.
func Generate(t *rapid.T, cfg Config) Program {
	m := newMachine(cfg)
	for range cfg.Rounds {
		t.Repeat(m.actions())
	}
	return m.finish(t)
}

// newMachine returns a machine with one empty package.
func newMachine(cfg Config) *machine {
	m := &machine{cfg: cfg, module: "example.com/generated", only: -1}
	m.openPackage()
	return m
}

// Programs returns the generator of programs for rapid.
func Programs(cfg Config) *rapid.Generator[Program] {
	return rapid.Custom(func(t *rapid.T) Program { return Generate(t, cfg) })
}

// Example returns the program of a seed. One seed gives one program, for fixed
// values of the flags -rapid.steps and -short.
func Example(seed int, cfg Config) Program {
	return Programs(cfg).Example(seed)
}

// actions returns the state machine's actions; a second name doubles an action's weight.
func (m *machine) actions() map[string]func(*rapid.T) {
	return map[string]func(*rapid.T){
		"":                 m.check,
		"OpenPackage":      m.OpenPackage,
		"DeclareStruct":    m.DeclareStruct,
		"DeclareInterface": m.DeclareInterface,
		"OpenFunc":         m.OpenFunc,
		"OpenFunc2":        m.OpenFunc,
		"OpenMethod":       m.OpenMethod,
		"OpenMethod2":      m.OpenMethod,
		"OpenGeneric":      m.OpenGeneric,
		"OpenRecursive":    m.OpenRecursive,
		"OpenImpure":       m.OpenImpure,
		"Declare":          m.Declare,
		"Declare2":         m.Declare,
		"Declare3":         m.Declare,
		"Assign":           m.Assign,
		"Assign2":          m.Assign,
		"Store":            m.Store,
		"Append":           m.Append,
		"OpenIf":           m.OpenIf,
		"Else":             m.Else,
		"OpenFor":          m.OpenFor,
		"OpenRange":        m.OpenRange,
		"Return":           m.Return,
		"Panic":            m.Panic,
		"Close":            m.Close,
		"Close2":           m.Close,
	}
}

// check holds after every action: blocks and variables exist only inside a function.
func (m *machine) check(t *rapid.T) {
	if m.fn == nil && (len(m.blocks) > 0 || len(m.vars) > 0 || m.body.Len() > 0) {
		t.Fatalf("blocks, variables or statements are left over outside a function")
	}
	if len(m.blocks) > maxBlocks {
		t.Fatalf("%d blocks are open, and the limit is %d", len(m.blocks), maxBlocks)
	}
}

// need skips the action when its precondition does not hold. It runs before any draw.
func need(t *rapid.T, ok bool) {
	if !ok {
		t.SkipNow()
	}
}

// pick draws an index below n. Index zero is the simplest choice, and shrinking moves toward it.
func pick(t *rapid.T, n int, label string) int {
	return rapid.IntRange(0, n-1).Draw(t, label)
}

// next returns a number that makes a name unique in the program.
func (m *machine) next() int {
	m.n++
	return m.n
}

// pkg returns the package being written.
func (m *machine) pkg() *pkg { return m.pkgs[len(m.pkgs)-1] }

// emit writes one line of the open function's body.
func (m *machine) emit(format string, args ...any) {
	fmt.Fprintf(&m.body, format+"\n", args...)
}

// feature records a construct that the open function uses.
func (m *machine) feature(name string) { m.fn.features[name] = true }

// inFunc reports whether a statement can be added to the open function.
func (m *machine) inFunc() bool { return m.fn != nil }

// inBranch reports whether the innermost block is a branch of an if statement.
// Only a branch may end with a return or a panic, so the code after it stays reachable.
func (m *machine) inBranch() bool {
	return m.fn != nil && len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind != "loop"
}

// loops returns the number of open loops.
func (m *machine) loops() (n int) {
	for _, b := range m.blocks {
		if b.kind == "loop" {
			n++
		}
	}
	return n
}

// openPackage starts the next package, which may import every earlier one.
func (m *machine) openPackage() {
	name := fmt.Sprintf("p%d", len(m.pkgs))
	m.pkgs = append(m.pkgs, &pkg{name: name, path: m.module + "/" + name, imports: map[string]bool{}})
}

// OpenPackage starts a new package once the current one has a function.
func (m *machine) OpenPackage(t *rapid.T) {
	need(t, m.fn == nil && len(m.pkg().funcs) > 0 && len(m.pkgs) < m.cfg.Packages)
	m.openPackage()
}

// DeclareStruct declares a struct type whose fields have basic types.
func (m *machine) DeclareStruct(t *rapid.T) {
	need(t, m.fn == nil && len(m.pkg().types) < 6)
	d := &typeDecl{pkg: m.pkg(), name: fmt.Sprintf("S%d", m.next()), methods: map[string]bool{}}
	d.typ, d.ptr = &typ{kind: kStruct, decl: d}, &typ{kind: kPointer, decl: d}
	var fields []string
	for i := range 1 + pick(t, 3, "fields") {
		f := field{name: fmt.Sprintf("F%d", i), typ: basics[pick(t, len(basics), "field type")]}
		d.fields = append(d.fields, f)
		fields = append(fields, f.name+" "+m.tname(f.typ))
	}
	m.pkg().types = append(m.pkg().types, d)
	m.pkg().decls = append(m.pkg().decls, fmt.Sprintf("type %s struct {\n%s\n}\n", d.name, strings.Join(fields, "\n")))
}

// DeclareInterface declares an interface type with one method, a func(int64) int64.
func (m *machine) DeclareInterface(t *rapid.T) {
	need(t, m.fn == nil && len(m.pkg().types) < 6)
	d := &typeDecl{pkg: m.pkg(), name: fmt.Sprintf("I%d", m.next()), method: fmt.Sprintf("M%d", m.next())}
	d.typ = &typ{kind: kIface, decl: d}
	m.pkg().types = append(m.pkg().types, d)
	m.pkg().decls = append(m.pkg().decls, fmt.Sprintf("type %s interface {\n%s(int64) int64\n}\n", d.name, d.method))
}

// visibleTypes returns the declared types the current package can name.
func (m *machine) visibleTypes(keep func(*typeDecl) bool) []*typeDecl {
	var out []*typeDecl
	for _, p := range m.pkgs {
		for _, d := range p.types {
			if keep(d) {
				out = append(out, d)
			}
		}
	}
	return out
}

// isStruct and isIface select the two sorts of declared types.
func isStruct(d *typeDecl) bool { return d.method == "" }
func isIface(d *typeDecl) bool  { return d.method != "" }

// anyType draws a type for a variable or a parameter.
func (m *machine) anyType(t *rapid.T) *typ {
	structs, ifaces := m.visibleTypes(isStruct), m.visibleTypes(isIface)
	switch c := pick(t, 10, "type"); {
	case c == 6 && len(structs) > 0:
		return structs[pick(t, len(structs), "struct")].typ
	case c == 7 && len(structs) > 0:
		return structs[pick(t, len(structs), "struct")].ptr
	case c == 8 && len(ifaces) > 0:
		return ifaces[pick(t, len(ifaces), "interface")].typ
	default:
		return []*typ{tInt, tBool, tString, tInts, tStrings, tMap, tInt, tString, tInt, tFunc}[c]
	}
}

// resultType draws a function's result type: a basic type, a slice, a map or a struct.
func (m *machine) resultType(t *rapid.T) *typ {
	structs := m.visibleTypes(isStruct)
	if c := pick(t, 7, "result"); c < 6 || len(structs) == 0 {
		return []*typ{tInt, tBool, tString, tInts, tStrings, tMap, tInt}[c]
	}
	return structs[pick(t, len(structs), "struct")].typ
}

// reaches reports whether a finished function that is not comptime is visible.
func (m *machine) reaches() bool {
	for _, p := range m.pkgs {
		for _, f := range p.funcs {
			if f.reach {
				return true
			}
		}
	}
	return false
}

// begin starts a function: its name, its export status and its empty body.
func (m *machine) begin(t *rapid.T, kind string) *fn {
	f := &fn{pkg: m.pkg(), kind: kind, features: map[string]bool{}, exported: pick(t, 4, "unexported") < 3}
	f.name = fmt.Sprintf("f%d", m.next())
	if f.exported {
		f.name = "F" + f.name[1:]
	}
	// A loose body may refer to functions that are not comptime, which makes it Unknown.
	f.loose = m.reaches() && pick(t, 3, "loose") == 2
	f.limit = maxCost
	m.fn, m.cost, m.vars, m.risky = f, 1, m.vars[:0], false
	return f
}

// param adds a parameter of the type to the open function.
func (m *machine) param(want *typ) variable {
	v := variable{name: fmt.Sprintf("a%d", len(m.fn.params)), typ: want, assignable: want.basic(), param: true}
	m.fn.params = append(m.fn.params, v)
	m.vars = append(m.vars, v)
	return v
}

// params adds up to three parameters of any type.
func (m *machine) params(t *rapid.T) {
	for range pick(t, 4, "params") {
		m.param(m.anyType(t))
	}
}

// OpenFunc opens a function with parameters and a result of any type.
func (m *machine) OpenFunc(t *rapid.T) {
	need(t, m.fn == nil)
	f := m.begin(t, "func")
	m.params(t)
	f.result = m.resultType(t)
}

// OpenMethod opens a method of a struct type of the current package. It may implement an interface.
func (m *machine) OpenMethod(t *rapid.T) {
	need(t, m.fn == nil)
	structs := m.visibleTypes(func(d *typeDecl) bool { return isStruct(d) && d.pkg == m.pkg() })
	need(t, len(structs) > 0)
	recv := structs[pick(t, len(structs), "receiver")]
	f := m.begin(t, "method")
	// Every method can be used as a func(int64) int64, so its cost is bounded like one.
	f.recv, f.exported, f.name, f.limit = recv, true, fmt.Sprintf("M%d", m.next()), ifaceCost
	ifaces := m.visibleTypes(func(d *typeDecl) bool { return isIface(d) && !recv.methods[d.method] })
	if c := pick(t, len(ifaces)+1, "implements"); c > 0 {
		f.implements, f.name = ifaces[c-1], ifaces[c-1].method
		recv.methods[f.name] = true
	}
	f.ptrRecv = rapid.Bool().Draw(t, "pointer receiver")
	r := variable{name: fmt.Sprintf("r%d", m.next()), typ: recv.typ, assignable: true}
	if f.ptrRecv {
		r.typ, r.assignable, r.param = recv.ptr, false, true
	}
	m.vars = append(m.vars, r)
	f.recvName = r.name
	m.param(tInt)
	f.result = tInt
	if m.planted < m.cfg.Impurities && pick(t, 4, "plant") == 3 {
		m.plant(t)
	}
}

// OpenGeneric opens a function with a type parameter T, a value of T and a func(T) int64.
func (m *machine) OpenGeneric(t *rapid.T) {
	need(t, m.fn == nil)
	f := m.begin(t, "generic")
	m.param(tParam)
	m.param(tPick)
	for range pick(t, 3, "params") {
		m.param(basics[pick(t, len(basics), "type")])
	}
	f.result = basics[pick(t, len(basics), "result")]
	m.feature("generic function")
}

// OpenRecursive opens a function that calls itself, or the first of two that call each other.
func (m *machine) OpenRecursive(t *rapid.T) {
	need(t, m.fn == nil)
	f := m.begin(t, "recursive")
	// The depth is never assigned, so the recursion ends after at most three calls.
	m.param(tInt)
	m.vars[0].assignable, f.params[0].assignable = false, false
	for range pick(t, 3, "params") {
		m.param(basics[pick(t, len(basics), "type")])
	}
	f.result = basics[pick(t, len(basics), "result")]
	if rapid.Bool().Draw(t, "pair") {
		g := &fn{pkg: f.pkg, kind: "recursive", features: map[string]bool{}, exported: f.exported, loose: f.loose}
		g.name = fmt.Sprintf("%c%d", f.name[0], m.next())
		g.params, g.result, g.partner, g.second = f.params, f.result, f, true
		f.partner = g
	}
	m.recursionGuard(t)
}

// recursionGuard writes the test that ends the recursion: the depth is at most three.
func (m *machine) recursionGuard(t *rapid.T) {
	m.feature("recursion")
	if m.fn.partner != nil {
		m.feature("mutual recursion")
	}
	m.emit("if a0 <= 0 || a0 > 3 {\nreturn %s\n}", m.expr(t, m.fn.result, 1))
}

// OpenImpure opens a function with a planted impurity.
func (m *machine) OpenImpure(t *rapid.T) {
	need(t, m.fn == nil && m.planted < m.cfg.Impurities)
	m.openImpure(t)
}

// openImpure opens a function whose result depends on an impurity, or that writes through a parameter.
func (m *machine) openImpure(t *rapid.T) {
	f := m.begin(t, "func")
	if m.only < 0 && pick(t, len(impurities)+1, "kind") == len(impurities) {
		shared := []*typ{tInts, tStrings, tMap}
		for _, d := range m.visibleTypes(isStruct) {
			shared = append(shared, d.ptr)
		}
		target := m.param(shared[pick(t, len(shared), "written parameter")])
		m.params(t)
		f.result = m.resultType(t)
		f.planted, f.impurity, f.loose = true, parameterWrite, false
		m.planted++
		m.writeThrough(t, target)
		return
	}
	m.params(t)
	// A bool has too few values to show every impurity, so the result is an int64 or a string.
	f.result = []*typ{tInt, tString}[pick(t, 2, "result")]
	m.plant(t)
}

// writeThrough writes to the variable a parameter refers to, when the argument has one.
func (m *machine) writeThrough(t *rapid.T, target variable) {
	switch target.typ.kind {
	case kSlice:
		m.emit("if len(%s) > 0 {\n%s[0] = %s\n}", target.name, target.name, m.expr(t, target.typ.elem, 1))
	case kMap:
		m.emit("if %s != nil {\n%s[%q] = %s\n}", target.name, target.name, "w", m.expr(t, tInt, 1))
	case kPointer:
		f := target.typ.decl.fields[0]
		m.emit("if %s != nil {\n%s.%s = %s\n}", target.name, target.name, f.name, m.expr(t, f.typ, 1))
	}
}

// plant makes the open function's result depend on a catalog impurity behind up to three indirections.
func (m *machine) plant(t *rapid.T) {
	f := m.fn
	imp := impurities[pick(t, len(impurities), "impurity")]
	if m.only >= 0 {
		imp = impurities[m.only]
	}
	id := fmt.Sprint(m.next())
	number := func(s string) string { return strings.ReplaceAll(s, "#", id) }
	for _, path := range imp.imports {
		f.pkg.imports[path] = true
	}
	hidden := "imp" + id
	decls := number(imp.decls) + "func " + hidden + "() int64 {\n" + number(imp.body) + "\n}\n\n"
	shows := imp.shows
	depth := pick(t, 4, "layers")
	if m.only >= 0 {
		depth = 0
	}
	for range depth {
		l := layers[pick(t, len(layers), "layer")]
		if l.name == initializerLayer {
			if imp.stateful {
				// Evaluated once at initialization, a package variable has a fixed value.
				continue
			}
			if shows == "repeat" {
				shows = "rerun"
			}
		}
		for _, path := range l.imports {
			f.pkg.imports[path] = true
		}
		d, name := l.wrap(m.next(), hidden)
		decls, hidden = decls+d, name
		f.layers = append(f.layers, l.name)
	}
	f.pkg.decls = append(f.pkg.decls, decls)
	if imp.between != "" {
		m.between = append(m.between, between{pkg: f.pkg, stmt: number(imp.between)})
	}
	f.taint = "t" + id
	// The rest of the body is comptime and cannot panic, so the impurity decides every result.
	f.planted, f.calm, f.loose, f.impurity, f.shows = true, true, false, imp.name, shows
	m.planted++
	m.emit("%s := %s()", f.taint, hidden)
}

// mix makes a result of the open function depend on its planted impure value.
func (m *machine) mix(e string) string {
	f := m.fn
	if f.taint == "" {
		return e
	}
	if f.result == tInt {
		return "(" + e + ") + " + f.taint
	}
	f.pkg.imports["strconv"] = true
	return m.clip("strconv.FormatInt(" + f.taint + ", 10) + " + e)
}

// clip bounds the length of a string, so that no loop can make one grow without limit.
func (m *machine) clip(e string) string {
	if clipped(e) {
		return e
	}
	m.cur().clip = true
	return "clip(" + e + ")"
}

// clipped reports whether an expression is one call of clip, or a string literal.
func clipped(e string) bool {
	if strings.HasPrefix(e, `"`) && strings.Count(e, `"`) == 2 {
		return true
	}
	if !strings.HasPrefix(e, "clip(") {
		return false
	}
	depth := 0
	for i, c := range e {
		switch c {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i == len(e)-1
			}
		}
	}
	return false
}

// declare adds a local variable and uses it once, because Go rejects an unused variable.
func (m *machine) declare(v variable, init string) {
	if v.typ.kind == kIface {
		m.emit("var %s %s = %s", v.name, m.tname(v.typ), init)
	} else {
		m.emit("%s := %s", v.name, init)
	}
	m.emit("_ = %s", v.name)
	m.vars = append(m.vars, v)
}

// Declare declares a local variable of any type.
func (m *machine) Declare(t *rapid.T) {
	need(t, m.inFunc())
	m.risky = false
	want := m.anyType(t)
	m.cost++
	v := variable{name: fmt.Sprintf("v%d", m.next()), typ: want, assignable: true}
	if want.shared() && pick(t, 3, "fresh") > 0 {
		// The function created what the variable refers to, and may write to it.
		v.owned, v.unique = true, true
		m.declare(v, m.construct(t, want))
		return
	}
	m.declare(v, m.expr(t, want, exprDepth))
}

// Assign assigns to a variable of a basic type.
func (m *machine) Assign(t *rapid.T) {
	need(t, m.inFunc())
	targets := m.find(func(v variable) bool { return v.assignable && v.typ.basic() })
	need(t, len(targets) > 0)
	m.risky = false
	v := m.vars[targets[pick(t, len(targets), "target")]]
	e := m.expr(t, v.typ, exprDepth)
	switch {
	case v.typ == tInt && rapid.Bool().Draw(t, "add"):
		m.emit("%s += %s", v.name, e)
	case v.typ == tString && rapid.Bool().Draw(t, "concatenate"):
		m.emit("%s = %s", v.name, m.clip(v.name+" + "+e))
	default:
		m.emit("%s = %s", v.name, e)
	}
	m.cost++
}

// Store writes to a field, a map entry or a slice element that the function created.
func (m *machine) Store(t *rapid.T) {
	need(t, m.inFunc())
	targets := m.find(func(v variable) bool {
		switch v.typ.kind {
		case kStruct:
			return v.assignable
		case kSlice:
			return v.owned && !m.fn.calm
		}
		return v.owned
	})
	need(t, len(targets) > 0)
	m.risky = false
	v := m.vars[targets[pick(t, len(targets), "target")]]
	switch v.typ.kind {
	case kStruct, kPointer:
		f := v.typ.decl.fields[pick(t, len(v.typ.decl.fields), "field")]
		m.emit("%s.%s = %s", v.name, f.name, m.expr(t, f.typ, exprDepth))
		m.feature("field write")
	case kMap:
		m.emit("%s[%s] = %s", v.name, m.expr(t, tString, 1), m.expr(t, tInt, exprDepth))
		m.feature("map write")
	case kSlice:
		m.panics("index")
		m.emit("%s[%d] = %s", v.name, pick(t, 4, "index"), m.expr(t, v.typ.elem, exprDepth))
		m.feature("slice write")
	}
	m.cost++
}

// Append appends to a slice that the function created and that has no alias.
func (m *machine) Append(t *rapid.T) {
	need(t, m.inFunc())
	targets := m.find(func(v variable) bool { return v.typ.kind == kSlice && v.owned && v.unique })
	need(t, len(targets) > 0)
	m.risky = false
	v := m.vars[targets[pick(t, len(targets), "target")]]
	m.emit("%s = append(%s, %s)", v.name, v.name, m.expr(t, v.typ.elem, exprDepth))
	m.feature("append")
	m.cost++
}

// open pushes a block.
func (m *machine) open(kind string, mult float64) {
	m.blocks = append(m.blocks, block{kind: kind, vars: len(m.vars), mult: mult, outer: m.cost})
	m.cost = 1
}

// OpenIf opens an if statement.
func (m *machine) OpenIf(t *rapid.T) {
	need(t, m.inFunc() && len(m.blocks) < maxBlocks)
	m.risky = false
	m.emit("if %s {", m.expr(t, tBool, exprDepth))
	m.open("if", 1)
	m.feature("if")
}

// Else turns the innermost if statement to its else branch.
func (m *machine) Else(t *rapid.T) {
	need(t, m.fn != nil && len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == "if")
	b := &m.blocks[len(m.blocks)-1]
	m.emit("} else {")
	m.vars, b.kind = m.vars[:b.vars], "else"
}

// OpenFor opens a loop with a literal bound.
func (m *machine) OpenFor(t *rapid.T) {
	need(t, m.inFunc() && len(m.blocks) < maxBlocks && m.loops() < maxLoops)
	i, bound := fmt.Sprintf("i%d", m.next()), 1+pick(t, 4, "bound")
	m.emit("for %s := int64(0); %s < %d; %s++ {", i, i, bound, i)
	m.open("loop", float64(bound))
	m.vars = append(m.vars, variable{name: i, typ: tInt})
	m.feature("for")
}

// OpenRange opens a loop over a slice, a string or an integer.
func (m *machine) OpenRange(t *rapid.T) {
	need(t, m.inFunc() && len(m.blocks) < maxBlocks && m.loops() < maxLoops)
	over := m.find(func(v variable) bool { return v.typ.kind == kSlice || v.typ == tString })
	id := m.next()
	i, x := fmt.Sprintf("i%d", id), fmt.Sprintf("x%d", id)
	m.feature("range")
	c := pick(t, len(over)+1, "over")
	if c == 0 {
		bound := 1 + pick(t, 4, "bound")
		m.emit("for %s := range int64(%d) {\n_ = %s", i, bound, i)
		m.open("loop", float64(bound))
		m.vars = append(m.vars, variable{name: i, typ: tInt})
		return
	}
	v := m.vars[over[c-1]]
	// The index of a range has type int, whose size is the target's; an int64 copy is used.
	// The break bounds the cost of the loop, whatever the length of the operand.
	m.emit("for %s_, %s_ := range %s {\nif %s_ >= %d {\nbreak\n}", i, x, v.name, i, rangeLimit)
	m.open("loop", rangeLimit)
	elem, conv := tInt, "int64(%s_)"
	if v.typ.kind == kSlice {
		elem, conv = v.typ.elem, "%s_"
	}
	m.emit("%s, %s := int64(%s_), "+conv+"\n_, _ = %s, %s", i, x, i, x, i, x)
	m.vars = append(m.vars, variable{name: i, typ: tInt}, variable{name: x, typ: elem})
}

// Return ends the innermost block with a return statement.
func (m *machine) Return(t *rapid.T) {
	need(t, m.inBranch())
	m.risky = false
	m.emit("return %s", m.mix(m.expr(t, m.fn.result, exprDepth)))
	m.feature("early return")
	m.leave(t)
}

// leave ends the innermost block after a return or a panic: nothing may follow in it.
func (m *machine) leave(t *rapid.T) {
	if m.blocks[len(m.blocks)-1].kind == "if" && rapid.Bool().Draw(t, "else") {
		m.Else(t)
		return
	}
	m.close(t)
}

// Panic ends the innermost block with a call of panic.
func (m *machine) Panic(t *rapid.T) {
	need(t, m.inBranch() && !m.fn.calm)
	m.risky = false
	m.panics("explicit panic")
	m.emit("panic(%s)", m.expr(t, tString, 1))
	m.leave(t)
}

// Close closes the innermost block, or the function when no block is open.
func (m *machine) Close(t *rapid.T) {
	need(t, m.fn != nil)
	m.close(t)
}

// close closes the innermost block or the open function.
func (m *machine) close(t *rapid.T) {
	if len(m.blocks) == 0 {
		m.closeFunc(t)
		return
	}
	b := m.blocks[len(m.blocks)-1]
	m.blocks = m.blocks[:len(m.blocks)-1]
	m.emit("}")
	m.vars, m.cost = m.vars[:b.vars], b.outer+b.mult*m.cost
}

// closeFunc writes the final return statement and adds the function to its package.
func (m *machine) closeFunc(t *rapid.T) {
	f := m.fn
	m.risky = f.kind == "recursive"
	result := m.expr(t, f.result, exprDepth)
	if f.kind == "recursive" {
		// The recursive call may panic, so nothing else in its statement may.
		m.risky = true
		result = m.recursiveResult(t, result)
		m.cost *= 4
	}
	m.emit("return %s", m.mix(result))
	f.cost = m.cost
	for _, g := range f.refs {
		f.reach = f.reach || g.reach
	}
	f.reach = f.reach || f.planted
	f.pkg.decls = append(f.pkg.decls, m.signature(f)+" {\n"+m.body.String()+"}\n")
	f.pkg.funcs = append(f.pkg.funcs, f)
	if f.recv != nil {
		f.recv.fns = append(f.recv.fns, f)
	}
	if f.implements != nil {
		f.implements.impls = append(f.implements.impls, f)
	}
	m.fn, m.vars, m.cost = nil, m.vars[:0], 0
	m.body.Reset()
	switch {
	case f.partner != nil && !f.second:
		m.openPartner(t, f.partner)
	case f.partner != nil:
		// Each of the two functions calls the other, so they share one label.
		reach, panics, cost := f.reach || f.partner.reach, f.panics || f.partner.panics, f.cost+f.partner.cost
		f.reach, f.partner.reach, f.panics, f.partner.panics = reach, reach, panics, panics
		f.cost, f.partner.cost = cost, cost
	}
}

// openPartner opens the second of two functions that call each other.
func (m *machine) openPartner(t *rapid.T, g *fn) {
	m.fn, m.cost, m.risky = g, 1, false
	m.vars = append(m.vars[:0], g.params...)
	m.recursionGuard(t)
}

// recursiveResult combines a result with that of the recursive call, one level down.
func (m *machine) recursiveResult(t *rapid.T, e string) string {
	f := m.fn
	callee := f
	if f.partner != nil {
		callee = f.partner
	}
	args := []string{"a0 - 1"}
	for _, p := range f.params[1:] {
		args = append(args, m.expr(t, p.typ, 1))
	}
	call := callee.name + "(" + strings.Join(args, ", ") + ")"
	f.refs = append(f.refs, callee)
	switch f.result {
	case tInt:
		return call + " + " + e
	case tBool:
		return call + " != " + e
	}
	return m.clip(call + " + " + e)
}

// signature returns the declaration of a function up to its body.
func (m *machine) signature(f *fn) string {
	var params []string
	for _, p := range f.params {
		params = append(params, p.name+" "+m.tname(p.typ))
	}
	head := "func " + f.name
	switch {
	case f.recv != nil && f.ptrRecv:
		head = "func (" + f.recvName + " *" + f.recv.name + ") " + f.name
	case f.recv != nil:
		head = "func (" + f.recvName + " " + f.recv.name + ") " + f.name
	case f.kind == "generic":
		head += "[T any]"
	}
	return head + "(" + strings.Join(params, ", ") + ") " + m.tname(f.result)
}
