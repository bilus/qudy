package qudy_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// compile compiles a template, or fails the test.
func compile(t *testing.T, src string) string {
	t.Helper()
	out, err := qudy.Compile("test.go", []byte(src), "g.generate", qudy.NewScope("sc", "newScope()", "fresh"))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// template wraps output lines in a function body.
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
			"an output line becomes a call",
			template("//`return nil"),
			"g.generate(`return nil\n`)",
		},
		{
			"a run of lines becomes one call",
			template("//`if x {", "//`\treturn nil", "//`}"),
			"g.generate(`if x {\n\treturn nil\n}\n`)",
		},
		{
			"gofmt's spelling of the mark is the same line",
			template("// `return nil"),
			"g.generate(`return nil\n`)",
		},
		{
			"an interpolation becomes an argument",
			template("//`return ~err"),
			"g.generate(`return %s\n`, err)",
		},
		{
			"scaffolding around a run is kept",
			template("for i := range xs {", "//`x := ~i", "}"),
			"for i := range xs {\n\t\tg.generate(`x := %s\n`, i)\n\t}",
		},
		{
			"an indented output line writes its text unindented",
			template("\tif x {", "\t\t//`return nil", "\t}"),
			"g.generate(`return nil\n`)",
		},
		{
			"a backtick in the text quotes the literal",
			template("//`x := " + "\x60raw\x60"),
			`g.generate("x := ` + "\x60raw\x60" + `\n")`,
		},
		{
			"a trailing backslash joins the next line",
			template("//`a(\\", "for range xs {", "//`x, \\", "}", "//`)"),
			"g.generate(`a(`)",
		},
		{
			"a percent in the text doubles",
			template("//`// 50% done"),
			"g.generate(`// 50%% done\n`)",
		},
		{
			"a built-in call becomes the scope's",
			template(`x := GENSYM("e")`, "_ = x"),
			"sc := newScope()\n\tx := sc.fresh(\"e\")",
		},
		{
			"two built-in calls on one line both become the scope's",
			template(`a, b := GENSYM("x"), GENSYM(p.Name)`, "_, _ = a, b"),
			`a, b := sc.fresh("x"), sc.fresh(p.Name)`,
		},
		{
			"the scope is declared where the names are used",
			template("//`a := errName#"),
			"func f() {\n\tsc := newScope()\n\terrName := sc.fresh(\"errName\")",
		},
		{
			"a template without a name of the generated program has no scope",
			template("//`a := 1"),
			"func f() {\n\tg.generate",
		},
		{
			"a name of the generated program is declared before its run",
			template("//`~x := errName#"),
			"errName := sc.fresh(\"errName\")\n\tg.generate(`%s := %s\n`, x, errName)",
		},
		{
			"a second run in the same block reuses the name",
			template("//`a := errName#", "x := 1", "//`b := errName#"),
			"errName := sc.fresh(\"errName\")\n\tg.generate(`a := %s\n`, errName)\n\tx := 1\n\tg.generate(`b := %s\n`, errName)",
		},
		{
			"a run in a loop declares the name inside it",
			template("for range xs {", "//`a := errName#", "}"),
			"for range xs {\n\t\terrName := sc.fresh(\"errName\")\n\t\tg.generate(`a := %s\n`, errName)\n\t}",
		},
		{
			"a sibling block declares its own name",
			template("if x {", "//`a := errName#", "}", "if y {", "//`b := errName#", "}"),
			"if y {\n\t\terrName := sc.fresh(\"errName\")\n\t\tg.generate(`b := %s\n`, errName)\n\t}",
		},
		{
			"a mark inside a block comment is not an output line",
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

func TestCompileWithoutAScopeRejectsTheNames(t *testing.T) {
	type testCase struct {
		description string
		template    string
	}
	for _, c := range []testCase{
		{"a name in an output line", template("//`a := errName#")},
		{"a built-in call", template(`x := GENSYM("e")`, "_ = x")},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate", qudy.Scope{})
			if err == nil || !strings.Contains(err.Error(), "the compiler was given no scope") {
				t.Fatalf("Compile answered %v, want a rejection naming the scope", err)
			}
		})
	}
}

func TestCompileWithAScopeTheTemplateDeclares(t *testing.T) {
	scope := qudy.NewScope("sc", "", "fresh")
	declared := template("sc := newScope(taken)", "//`a := errName#")
	out, err := qudy.Compile("test.go", []byte(declared), "g.generate", scope)
	if err != nil {
		t.Fatal(err)
	}
	if want := "sc := newScope(taken)\n\terrName := sc.fresh(\"errName\")"; !strings.Contains(string(out), want) {
		t.Errorf("compiled to\n%s\nwant it to hold\n%s", out, want)
	}
	_, err = qudy.Compile("test.go", []byte(template("//`a := errName#")), "g.generate", scope)
	if err == nil || !strings.Contains(err.Error(), "neither the template nor the scope creates sc") {
		t.Fatalf("Compile answered %v, want a rejection naming the missing scope", err)
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
		{"an output line outside a function", "package p\n\n//`x := 1\nvar y = 1\n", "belongs inside a function body"},
		{"a comment of the template that opens with the mark", "package p\n\n// `x` is a name\nfunc f() {}\n", "belongs inside a function body"},
		{"a template that declares a built-in", "package p\n\nfunc GENSYM(s string) string { return s }\n", "which is a built-in"},
		{"a name the template declares", "package p\n\nfunc f() {\n\terrName := 1\n\t_ = errName\n\t//`x := errName#\n}\n", "which errName# would shadow"},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate", qudy.NewScope("sc", "newScope()", "fresh"))
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Compile answered %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
