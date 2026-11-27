package gen

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"pgregory.net/rapid"
)

// clipDecl bounds the length of the strings a generated package computes.
var clipDecl = fmt.Sprintf("func clip(s string) string {\nif len(s) > %d {\nreturn s[:%d]\n}\nreturn s\n}\n", clipLen, clipLen)

// callDecl prints the result of one call, or the value it panicked with.
const callDecl = `func call(name string, f func() any) {
defer func() {
if r := recover(); r != nil {
fmt.Printf("%s panic: %v\n", name, r)
}
}()
fmt.Printf("%s %#v\n", name, f())
}
`

// finish closes what is open, adds package main, and returns the program with its labels.
func (m *machine) finish(t *rapid.T) Program {
	for m.fn != nil {
		m.close(t)
	}
	if m.planted < 1 && m.cfg.Impurities > 0 {
		m.openImpure(t)
		m.closeFunc(t)
	}
	if len(m.pkgs) == 1 && len(m.pkgs[0].funcs) == 0 {
		m.begin(t, "func").result = tInt
		m.closeFunc(t)
	}
	lib := m.pkgs
	m.addMain(t)
	p := Program{Module: m.module}
	for _, k := range m.pkgs {
		file := k.name + "/" + k.name + ".go"
		if k.name == "main" {
			file = "main.go"
		}
		p.Packages = append(p.Packages, Package{Path: k.path, Name: k.name, File: file, Src: render(t, k)})
	}
	for _, k := range lib {
		for _, f := range k.funcs {
			p.Symbols = append(p.Symbols, symbolOf(f))
		}
	}
	return p
}

// addMain adds package main, which calls every exported function twice and prints the results.
func (m *machine) addMain(t *rapid.T) {
	lib := m.pkgs
	k := &pkg{name: "main", path: m.module, imports: map[string]bool{"fmt": true}}
	m.pkgs = append(m.pkgs, k)
	m.fn = &fn{pkg: k, name: "main", kind: "main", limit: 1e18, features: map[string]bool{}}
	var round strings.Builder
	for _, p := range lib {
		for _, f := range p.funcs {
			if !f.exported {
				continue
			}
			// The arguments themselves never panic, so a panic comes from the function called.
			m.risky, m.cost = true, 0
			fmt.Fprintf(&round, "call(%q, func() any { return %s })\n", p.name+"."+f.symbol(), m.call(t, f, 1))
		}
	}
	var assigns []string
	for _, b := range m.between {
		assigns = append(assigns, m.qualify(b.pkg, b.stmt))
	}
	m.fn = nil
	k.decls = append(k.decls,
		callDecl,
		"func round() {\n"+round.String()+"}\n",
		"func main() {\nround()\nfmt.Println(\"--\")\n"+strings.Join(assigns, "\n")+"\nround()\n}\n")
}

// render returns the gofmt-formatted source of a package.
func render(t *rapid.T, k *pkg) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", k.name)
	if len(k.imports) > 0 {
		paths := make([]string, 0, len(k.imports))
		for path := range k.imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		b.WriteString("import (\n")
		for _, path := range paths {
			fmt.Fprintf(&b, "%q\n", path)
		}
		b.WriteString(")\n\n")
	}
	if k.clip {
		b.WriteString(clipDecl + "\n")
	}
	b.WriteString(strings.Join(k.decls, "\n"))
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		t.Fatalf("package %s is not Go: %v\n%s", k.name, err, b.String())
	}
	return string(src)
}

// symbolOf returns a finished function with its label and the reason for it.
func symbolOf(f *fn) Symbol {
	s := Symbol{Package: f.pkg.path, Name: f.symbol(), Exported: f.exported, Label: f.label(), Layers: f.layers}
	for name := range f.features {
		s.Features = append(s.Features, name)
	}
	sort.Strings(s.Features)
	seen := map[*fn]bool{}
	for g := f; g != nil && !seen[g]; {
		seen[g] = true
		if g.planted {
			s.Because = append(s.Because, g.impurity)
			break
		}
		var next *fn
		for _, r := range g.refs {
			if r.reach && !seen[r] {
				next = r
				break
			}
		}
		if next != nil {
			s.Because = append(s.Because, next.pkg.name+"."+next.symbol())
		}
		g = next
	}
	if f.planted {
		s.Impurity, s.Shows = f.impurity, f.shows
	}
	return s
}
