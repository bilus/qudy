package qudy_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// symbols is the symbol generator every test compiles with.
var symbols = qudy.NewSymbolGenerator("symbols", "newSymbolGenerator()", "gensym")

// compile compiles a template, or fails the test.
func compile(t *testing.T, src string) string {
	t.Helper()
	gen, err := qudy.Compile("test.go", []byte(src), "g.generate", symbols)
	if err != nil {
		t.Fatal(err)
	}
	return string(gen)
}

// template wraps lines in a function body.
func template(lines ...string) string {
	return "package p\n\nfunc f() {\n" + strings.Join(lines, "\n") + "\n}\n"
}

func TestCompileWritesTheEmitCalls(t *testing.T) {
	type testCase struct {
		description string
		template    string
		want        string
	}
	for _, c := range []testCase{
		{
			"an output line becomes an emit call",
			template("//`return nil"),
			"g.generate(`return nil\n`)",
		},
		{
			"a run becomes one emit call",
			template("//`if x {", "//`\treturn nil", "//`}"),
			"g.generate(`if x {\n\treturn nil\n}\n`)",
		},
		{
			"a space before the backtick gives the same output line",
			template("// `return nil"),
			"g.generate(`return nil\n`)",
		},
		{
			"an interpolation becomes an argument",
			template("//`return ~err"),
			"g.generate(`return %s\n`, err)",
		},
		{
			"the compiler keeps the code around a run",
			template("for i := range xs {", "//`x := ~i", "}"),
			"for i := range xs {\n\t\tg.generate(`x := %s\n`, i)\n\t}",
		},
		{
			"an indented output line writes its text without the indent",
			template("\tif x {", "\t\t//`return nil", "\t}"),
			"g.generate(`return nil\n`)",
		},
		{
			"a backtick in the text gives an interpreted string literal",
			template("//`x := " + "\x60raw\x60"),
			`g.generate("x := ` + "\x60raw\x60" + `\n")`,
		},
		{
			"a trailing backslash joins the next output line",
			template("//`a(\\", "for range xs {", "//`x, \\", "}", "//`)"),
			"g.generate(`a(`)",
		},
		{
			"the compiler doubles a percent sign in the text",
			template("//`// 50% done"),
			"g.generate(`// 50%% done\n`)",
		},
		{
			"a GENSYM call becomes a call to the symbol generator's method",
			template(`x := GENSYM("e")`, "_ = x"),
			"symbols := newSymbolGenerator()\n\tx := symbols.gensym(\"e\")",
		},
		{
			"two GENSYM calls on one line both become calls to the symbol generator's method",
			template(`a, b := GENSYM("x"), GENSYM(p.Name)`, "_, _ = a, b"),
			`a, b := symbols.gensym("x"), symbols.gensym(p.Name)`,
		},
		{
			"a function body with a gensym declares the symbol generator",
			template("//`a := errName#"),
			"func f() {\n\tsymbols := newSymbolGenerator()\n\terrName := symbols.gensym(\"errName\")",
		},
		{
			"a template without a gensym declares no symbol generator",
			template("//`a := 1"),
			"func f() {\n\tg.generate",
		},
		{
			"the compiler declares a gensym before its run",
			template("//`~x := errName#"),
			"errName := symbols.gensym(\"errName\")\n\tg.generate(`%s := %s\n`, x, errName)",
		},
		{
			"a second run in the same block reuses the gensym",
			template("//`a := errName#", "x := 1", "//`b := errName#"),
			"errName := symbols.gensym(\"errName\")\n\tg.generate(`a := %s\n`, errName)\n\tx := 1\n\tg.generate(`b := %s\n`, errName)",
		},
		{
			"a run in a loop declares the gensym inside the loop",
			template("for range xs {", "//`a := errName#", "}"),
			"for range xs {\n\t\terrName := symbols.gensym(\"errName\")\n\t\tg.generate(`a := %s\n`, errName)\n\t}",
		},
		{
			"a sibling block declares its own gensym",
			template("if x {", "//`a := errName#", "}", "if y {", "//`b := errName#", "}"),
			"if y {\n\t\terrName := symbols.gensym(\"errName\")\n\t\tg.generate(`b := %s\n`, errName)\n\t}",
		},
		{
			"a line inside a block comment is not an output line",
			template("/*", "//`x := 1", "*/"),
			"//`x := 1",
		},
	} {
		t.Run(c.description, func(t *testing.T) {
			if got := compile(t, c.template); !strings.Contains(got, c.want) {
				t.Errorf("compiled to\n%s\nwant it to hold\n%s", got, c.want)
			}
		})
	}
}

func TestCompileWithoutASymbolGeneratorRejectsAGensym(t *testing.T) {
	type testCase struct {
		description string
		template    string
	}
	for _, c := range []testCase{
		{"a gensym in an output line", template("//`a := errName#")},
		{"a GENSYM call", template(`x := GENSYM("e")`, "_ = x")},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate", qudy.SymbolGenerator{})
			if err == nil || !strings.Contains(err.Error(), "the compiler was given no symbol generator") {
				t.Fatalf("Compile returned %v, want a rejection naming the symbol generator", err)
			}
		})
	}
}

func TestCompileWhenTheTemplateDeclaresTheSymbolGenerator(t *testing.T) {
	withoutCreate := qudy.NewSymbolGenerator("symbols", "", "gensym")
	declared := template("symbols := newSymbolGenerator(taken)", "//`a := errName#")
	gen, err := qudy.Compile("test.go", []byte(declared), "g.generate", withoutCreate)
	if err != nil {
		t.Fatal(err)
	}
	if want := "symbols := newSymbolGenerator(taken)\n\terrName := symbols.gensym(\"errName\")"; !strings.Contains(string(gen), want) {
		t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
	}
	_, err = qudy.Compile("test.go", []byte(template("//`a := errName#")), "g.generate", withoutCreate)
	if err == nil || !strings.Contains(err.Error(), "the template does not declare symbols") {
		t.Fatalf("Compile returned %v, want a rejection naming the missing symbol generator", err)
	}
}

func TestCompileRejects(t *testing.T) {
	type testCase struct {
		description string
		template    string
		rejects     string
	}
	for _, c := range []testCase{
		{"a template that is not Go", "package p\n\nfunc f( {\n//`x\n}\n", "the template is not Go"},
		{"an interpolation without a name", template("//`x := ~1"), "starts with a name"},
		{"an output line after code on its line", template("count := compute() //`total := ~count", "_ = count"), "stands alone on its line"},
		{"an output line outside a function body", "package p\n\n//`x := 1\nvar y = 1\n", "belongs inside a function body"},
		{"a doc comment that opens with a backtick", "package p\n\n// `x` is a name\nfunc f() {}\n", "belongs inside a function body"},
		{"a template that declares GENSYM", "package p\n\nfunc GENSYM(s string) string { return s }\n", "which is a built-in function"},
		{"a GENSYM call outside a function body", "package p\n\nvar x = GENSYM(\"x\")\n", "a GENSYM call belongs inside a function body"},
		{"a gensym with the name of a template variable", "package p\n\nfunc f() {\n\terrName := 1\n\t_ = errName\n\t//`x := errName#\n}\n", "errName# would shadow"},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate", symbols)
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Compile returned %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
