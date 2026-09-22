package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// compile compiles a template and returns its generator without line directives.
func compile(t *testing.T, src string) string {
	t.Helper()
	gen, err := qudy.Compile("test.go", []byte(src), "g.generate")
	if err != nil {
		t.Fatal(err)
	}
	return withoutDirectives(string(gen))
}

// lineComments matches a line directive on its own line or inline.
var lineComments = regexp.MustCompile(`(?m)^//line [^\n]*\n| ?/\*line [^*]*\*/`)

// withoutDirectives returns a generator's text without its line directives.
func withoutDirectives(gen string) string {
	return lineComments.ReplaceAllString(gen, "")
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
			"g.generate(`return %v\n`, err)",
		},
		{
			"the compiler keeps the code around a run",
			template("for i := range xs {", "//`x := ~i", "}"),
			"for i := range xs {\n\t\tg.generate(`x := %v\n`, i)\n\t}",
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
			"a qudyGensym call stays as written, above the symbol generator's declaration",
			template(`x := qudyGensym("e")`, "_ = x"),
			"x := qudyGensym(\"e\")",
		},
		{
			"two qudyGensym calls on one line stay as written",
			template(`a, b := qudyGensym("x"), qudyGensym(p.Name)`, "_, _ = a, b"),
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
			"errName_qd := qudyGensym(\"errName\")\n\tg.generate(`%v := %v\n`, x, errName_qd)",
		},
		{
			"a second run in the same block reuses the gensym",
			template("//`a := errName#", "x := 1", "//`b := errName#"),
			"errName_qd := qudyGensym(\"errName\")\n\tg.generate(`a := %v\n`, errName_qd)\n\tx := 1\n\tg.generate(`b := %v\n`, errName_qd)",
		},
		{
			"a run in a loop declares the gensym inside the loop",
			template("for range xs {", "//`a := errName#", "}"),
			"for range xs {\n\t\terrName_qd := qudyGensym(\"errName\")\n\t\tg.generate(`a := %v\n`, errName_qd)\n\t}",
		},
		{
			"a sibling block declares its own gensym",
			template("if x {", "//`a := errName#", "}", "if y {", "//`b := errName#", "}"),
			"if y {\n\t\terrName_qd := qudyGensym(\"errName\")\n\t\tg.generate(`b := %v\n`, errName_qd)\n\t}",
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

// twelve writes twelve gensyms, enough for two digits, and interpolates an int.
const twelve = `package main

import "fmt"

func main() {
	for i := range 12 {
		//` + "`" + `e# := ~i
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
	if want := "e_qd1 := 0\ne_qd2 := 1\n"; !strings.HasPrefix(string(out), want) {
		t.Errorf("the generator wrote\n%s\nwant it to start with\n%s", out, want)
	}
	if want := "e_qd12 := 11\n"; !strings.HasSuffix(string(out), want) {
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

func TestCompileDeclaresTheSymbolGeneratorForADirectCall(t *testing.T) {
	gen := compile(t, template(`x := qudyGensym("e")`, "_ = x"))
	declared, called := strings.Index(gen, builtin), strings.Index(gen, `x := qudyGensym("e")`)
	if declared < 0 || called < declared {
		t.Errorf("the generator calls qudyGensym without a declaration above the call:\n%s", gen)
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
			if want := `errName_qd := qudyGensym("errName")`; !strings.Contains(gen, want) {
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

func TestCompileKeepsAGensymApartFromTheNamesOfTheTemplate(t *testing.T) {
	type testCase struct {
		description string
		template    string
	}
	use := "func f() {\n\t//`x := err#\n}\n"
	for _, c := range []testCase{
		{"a variable of the same function", "package p\n\nfunc f() {\n\terr := 1\n\t_ = err\n\t//`x := err#\n}\n"},
		{"a parameter of the same function", "package p\n\nfunc f(err error) {\n\t//`x := err#\n}\n"},
		{"a variable in a function literal", "package p\n\nfunc f() {\n\teach(func() {\n\t\terr := 1\n\t\t_ = err\n\t})\n\t//`x := err#\n}\n"},
		{"a function of the package", "package p\n\nfunc err() {}\n\n" + use},
		{"a variable of another function", "package p\n\nfunc other() {\n\terr := 1\n\t_ = err\n}\n\n" + use},
	} {
		t.Run(c.description, func(t *testing.T) {
			gen := compile(t, c.template)
			for _, want := range []string{`err_qd := qudyGensym("err")`, "g.generate(`x := %v\n`, err_qd)"} {
				if !strings.Contains(gen, want) {
					t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
				}
			}
		})
	}
}

// both writes a template variable beside the gensym of its name, without importing fmt.
const both = `package main

func main() {
	err := "theirs"
	//` + "`" + `~err and err#
}
`

func TestAGensymAndAVariableOfTheSameNameBothWork(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	gen, err := qudy.Compile("both.go", []byte(both), "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "both_gen.go")
	if err := os.WriteFile(file, gen, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), goTool, "run", file).CombinedOutput()
	if err != nil {
		t.Fatalf("the generator fails: %v\n%s", err, out)
	}
	if want := "theirs and err_qd1\n"; string(out) != want {
		t.Errorf("the generator wrote %q, want %q", out, want)
	}
}

func TestCompileImportsFmtForItsEmitCalls(t *testing.T) {
	type testCase struct {
		description string
		template    string
		emit        string
		imports     int
	}
	for _, c := range []testCase{
		{"a template without the import", template("//`return nil"), "fmt.Printf", 1},
		{"a template with the import", "package p\n\nimport \"fmt\"\n\nfunc f() {\n\t//`return nil\n}\n", "fmt.Printf", 1},
		{"a template with the import under another name", "package p\n\nimport f \"fmt\"\n\nfunc g() {\n\tf.Println()\n\t//`return nil\n}\n", "fmt.Printf", 2},
		{"an emit function outside fmt", template("//`return nil"), "g.generate", 0},
		{"a template without an output line", template("x := 1", "_ = x"), "fmt.Printf", 0},
	} {
		t.Run(c.description, func(t *testing.T) {
			gen, err := qudy.Compile("test.go", []byte(c.template), c.emit)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(gen), `"fmt"`); n != c.imports {
				t.Errorf("the generator imports fmt %d times, want %d:\n%s", n, c.imports, gen)
			}
		})
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
		{"a template with a name that ends in the reserved suffix", "package p\n\nfunc f() {\n\terr_qd := 1\n\t_ = err_qd\n}\n", "qudy reserves names that end in _qd"},
		{"a call of the former name of qudyGensym", template(`x := GENSYM("e")`, "_ = x"), "test.go:4: GENSYM is called qudyGensym now"},
		{"a qudyGensym call outside a function body", "package p\n\nvar x = qudyGensym(\"x\")\n", "test.go:3: a qudyGensym call outside a function body needs -runtime"},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(c.template), "g.generate")
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Compile returned %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
