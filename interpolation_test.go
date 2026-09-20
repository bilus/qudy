package qudy_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

func TestCompileReadsEachInterpolation(t *testing.T) {
	type testCase struct {
		description string
		line        string
		want        string
	}
	for _, c := range []testCase{
		{"text alone", "return nil", "g.generate(`return nil\n`)"},
		{"a name", "return ~err", "g.generate(`return %s\n`, err)"},
		{"a name with selectors", "x := ~p.Type.Text", "g.generate(`x := %s\n`, p.Type.Text)"},
		{"a call needs braces", "~{strconv.Quote(name)}: {", "g.generate(`%s: {\n`, strconv.Quote(name))"},
		{"a paren is the generated program's", "is~name() {", "g.generate(`is%s() {\n`, name)"},
		{"a bracket is the generated program's", "~args[0] = x", "g.generate(`%s[0] = x\n`, args)"},
		{"an index expression needs braces", "~{fns[0]}(a) = x", "g.generate(`%s(a) = x\n`, fns[0])"},
		{"a type parameter list is the generated program's", `func F~name[R any](\`, "g.generate(`func F%s[R any](`, name)"},
		{"a gensym", "if errName# != nil {", "g.generate(`if %s != nil {\n`, errName)"},
		{"two gensyms", "local#, errName# := f()", "g.generate(`%s, %s := f()\n`, local, errName)"},
		{"a doubled hash is a hash", "id##1 := ~n", "g.generate(`id#1 := %s\n`, n)"},
		{"two interpolations", "~a, ~b := f()", "g.generate(`%s, %s := f()\n`, a, b)"},
		{"braces take any expression", "x := ~{a + b}", "g.generate(`x := %s\n`, a+b)"},
		{"a doubled tilde is a tilde", "a ~~ b", "g.generate(`a ~ b\n`)"},
		{"the compiler drops a tilde comment and the spaces before it", "x := ~n   ~// why", "g.generate(`x := %s\n`, n)"},
		{"a comment without a tilde stays", "// DispatchTable maps names", "g.generate(`// DispatchTable maps names\n`)"},
		{"a tilde comment on its own leaves an empty line", "~// a note", "g.generate(`\n`)"},
		{"the compiler doubles a percent sign", "50% of ~n", "g.generate(`50%% of %s\n`, n)"},
		{"a brace inside a string literal does not close the interpolation", `~{q("a}b")} x`, "g.generate(`%s x\n`, q(\"a}b\"))"},
	} {
		t.Run(c.description, func(t *testing.T) {
			if got := compile(t, template("//`"+c.line)); !strings.Contains(got, c.want) {
				t.Errorf("compiled to\n%s\nwant it to hold\n%s", got, c.want)
			}
		})
	}
}

func TestCompileRejectsAnInterpolation(t *testing.T) {
	type testCase struct {
		description string
		line        string
		rejects     string
	}
	for _, c := range []testCase{
		{"without a name", "x := ~1", "starts with a name"},
		{"without a closing brace", "x := ~{a + b", "has no closing brace"},
		{"at the end of a line", "x := ~", "starts with a name"},
		{"a hash without a name before it", "x := #n", "follows the name of a gensym"},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(template("//`"+c.line)), "g.generate", symbols)
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Compile returned %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
