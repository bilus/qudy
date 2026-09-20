package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// compile compiles a template, or fails the test.
func compile(t *testing.T, src string) string {
	t.Helper()
	gen, err := qudy.Compile("test.go", []byte(src), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	return string(gen)
}

// builtin opens the declaration of the built-in symbol generator.
const builtin = "qudyGensym := func() func(string) string {"

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
			"a GENSYM call becomes a call to the symbol generator",
			template(`x := GENSYM("e")`, "_ = x"),
			"x := qudyGensym(\"e\")",
		},
		{
			"two GENSYM calls on one line both become calls to the symbol generator",
			template(`a, b := GENSYM("x"), GENSYM(p.Name)`, "_, _ = a, b"),
			`a, b := qudyGensym("x"), qudyGensym(p.Name)`,
		},
		{
			"a function body with a gensym declares the symbol generator",
			template("//`a := errName#"),
			"func f() {\n\tqudyGensym := func() func(string) string {",
		},
		{
			"a template without a gensym declares no symbol generator",
			template("//`a := 1"),
			"func f() {\n\tg.generate",
		},
		{
			"the compiler declares a gensym before its run",
			template("//`~x := errName#"),
			"errName := qudyGensym(\"errName\")\n\tg.generate(`%s := %s\n`, x, errName)",
		},
		{
			"a second run in the same block reuses the gensym",
			template("//`a := errName#", "x := 1", "//`b := errName#"),
			"errName := qudyGensym(\"errName\")\n\tg.generate(`a := %s\n`, errName)\n\tx := 1\n\tg.generate(`b := %s\n`, errName)",
		},
		{
			"a run in a loop declares the gensym inside the loop",
			template("for range xs {", "//`a := errName#", "}"),
			"for range xs {\n\t\terrName := qudyGensym(\"errName\")\n\t\tg.generate(`a := %s\n`, errName)\n\t}",
		},
		{
			"a sibling block declares its own gensym",
			template("if x {", "//`a := errName#", "}", "if y {", "//`b := errName#", "}"),
			"if y {\n\t\terrName := qudyGensym(\"errName\")\n\t\tg.generate(`b := %s\n`, errName)\n\t}",
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

// twelve writes twelve gensyms, enough for a number with two digits.
const twelve = `package main

import "fmt"

func main() {
	for range 12 {
		//` + "`" + `e# := 0
	}
}
`

func TestCompiledGeneratorRunsAsASingleFile(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	gen, err := qudy.Compile("twelve.go", []byte(twelve), "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "twelve_gen.go")
	if err := os.WriteFile(file, gen, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), goTool, "run", file).CombinedOutput()
	if err != nil {
		t.Fatalf("the generator fails: %v\n%s", err, out)
	}
	if want := "e_qd1 := 0\ne_qd2 := 0\n"; !strings.HasPrefix(string(out), want) {
		t.Errorf("the generator wrote\n%s\nwant it to start with\n%s", out, want)
	}
	if want := "e_qd12 := 0\n"; !strings.HasSuffix(string(out), want) {
		t.Errorf("the generator wrote\n%s\nwant it to end with\n%s", out, want)
	}
}

func TestTwoCompiledGeneratorsBuildInOnePackage(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		src := "package gen\n\nimport \"fmt\"\n\nfunc " + name + "() {\n\t//`e# := 0\n}\n"
		gen, err := qudy.Compile(name+".go", []byte(src), "fmt.Printf")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+"_gen.go"), gen, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	vet := exec.CommandContext(t.Context(), goTool, "vet", "./...")
	vet.Dir = dir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("the two generators do not build together: %v\n%s", err, out)
	}
}

func TestCompileUsesTheSymbolGeneratorOfTheTemplate(t *testing.T) {
	type testCase struct {
		description string
		template    string
	}
	for _, c := range []testCase{
		{"declared in the function body", template("qudyGensym := seeded(taken)", "//`a := errName#")},
		{"declared as a parameter", "package p\n\nfunc f(qudyGensym func(string) string) {\n//`a := errName#\n}\n"},
		{"declared at package level", "package p\n\nvar qudyGensym = seeded(nil)\n\nfunc f() {\n//`a := errName#\n}\n"},
		{"declared in the function body around a function literal", template("qudyGensym := seeded(taken)", "each(func() {", "//`a := errName#", "})")},
	} {
		t.Run(c.description, func(t *testing.T) {
			gen := compile(t, c.template)
			if strings.Contains(gen, builtin) {
				t.Errorf("the compiler declared its own symbol generator:\n%s", gen)
			}
			if want := `errName := qudyGensym("errName")`; !strings.Contains(gen, want) {
				t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
			}
		})
	}
}

func TestCompileDeclaresOneSymbolGeneratorForAFunctionLiteral(t *testing.T) {
	gen := compile(t, template("each(func() {", "//`a := errName#", "})"))
	if n := strings.Count(gen, builtin); n != 1 {
		t.Fatalf("the compiler declared %d symbol generators, want 1:\n%s", n, gen)
	}
	if strings.Index(gen, builtin) > strings.Index(gen, "each(func() {") {
		t.Errorf("the declaration follows the function literal:\n%s", gen)
	}
}

func TestCompileNegatesTheTemplateTag(t *testing.T) {
	type testCase struct {
		description string
		constraint  string
		want        string
	}
	for _, c := range []testCase{
		{"the tag alone", "//go:build qudy", "//go:build !qudy\n\npackage p"},
		{"the tag beside another", "//go:build qudy && linux", "//go:build !qudy && linux\n\npackage p"},
		{"a constraint without the tag stays", "//go:build linux", "//go:build linux\n\npackage p"},
		{"the tag beside the tag of go generate", "//go:build qudy || generate", "//go:build !qudy || generate\n\npackage p"},
	} {
		t.Run(c.description, func(t *testing.T) {
			gen := compile(t, c.constraint+"\n\n"+template("//`return nil"))
			if !strings.HasPrefix(gen, c.want) {
				t.Errorf("compiled to\n%s\nwant it to start with\n%s", gen, c.want)
			}
		})
	}
}

func TestCompileDropsTheGenerateDirectiveOfTheTemplate(t *testing.T) {
	src := "package p\n\n//go:generate qudy -o p_gen.go p.go\n\nfunc f() {\n\t//`return nil\n}\n"
	gen := compile(t, src)
	if strings.Contains(gen, "go:generate") {
		t.Errorf("the generator kept the directive:\n%s", gen)
	}
	if want := "package p\n\nfunc f() {"; !strings.Contains(gen, want) {
		t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
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
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate")
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Compile returned %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
