package qudy_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// loop returns the generator's loop for a splice of xs around an element's call.
func loop(element string) string {
	return "for qudyI, qudyX := range xs {\n\t\tif qudyI > 0 {\n\t\t\tg.generate(\", \")\n\t\t}\n\t\t" + element + "\n\t}"
}

func TestCompileSplicesAndQuotes(t *testing.T) {
	type testCase struct {
		description string
		line        string
		want        []string
	}
	for _, c := range []testCase{
		{"a quoted interpolation", `return ~"name`, []string{"g.generate(`return %q\n`, fmt.Sprint(name))"}},
		{"a quoted braced interpolation", `x := ~"{s.id}`, []string{"g.generate(`x := %q\n`, fmt.Sprint(s.id))"}},
		{"a splice between text", "f(~@xs)", []string{"g.generate(`f(`)", loop(`g.generate("%v", qudyX)`), "g.generate(`)\n`)"}},
		{"a quoted splice", `[]string{~@"xs}`, []string{"g.generate(`[]string{`)", loop(`g.generate("%q", fmt.Sprint(qudyX))`), "g.generate(`}\n`)"}},
		{"a braced splice", "f(~@{names(xs)})", []string{"for qudyI, qudyX := range names(xs) {"}},
		{"a splice at the end of a line", "return ~@xs", []string{"g.generate(`return `)", loop(`g.generate("%v", qudyX)`), "g.generate(`\n`)"}},
		{"a splice before a trailing backslash", `~@xs\`, []string{loop(`g.generate("%v", qudyX)`) + "\n}"}},
		{"text and arguments on both sides of a splice", "~a(~@xs) ~b", []string{"g.generate(`%v(`, a)", "g.generate(`) %v\n`, b)"}},
	} {
		t.Run(c.description, func(t *testing.T) {
			gen := compile(t, template("//`"+c.line))
			for _, want := range c.want {
				if !strings.Contains(gen, want) {
					t.Errorf("compiled to\n%s\nwant it to hold\n%s", gen, want)
				}
			}
		})
	}
}

func TestCompileRejectsASpliceOrAQuoteWithoutAnOperand(t *testing.T) {
	type testCase struct {
		description string
		line        string
	}
	for _, c := range []testCase{
		{"a splice before a space", "f(~@ xs)"},
		{"a quote before a space", `x := ~" y`},
		{"a quote before a splice", `x := ~"@xs`},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := qudy.Compile("test.go", []byte(template("//`"+c.line)), "g.generate")
			if err == nil || !strings.Contains(err.Error(), "starts with a name") {
				t.Fatalf("Compile returned %v, want a rejection naming the missing name", err)
			}
		})
	}
}

// lists writes an argument list, quoted lists and literals, and an empty list.
var lists = strings.Join([]string{
	"package main",
	"",
	"func main() {",
	`	params := []string{"a int", "b string"}`,
	`	statics := []string{"<p>", "say \"hi\"", "C:\\temp"}`,
	"	var none []int",
	`	path := "C:\\temp"`,
	"	count := 3",
	"	//`func f(~@params) {}",
	"	//`var statics = []string{~@\"statics}",
	"	//`var none = []int{~@none}",
	"	//`var quoted = ~\"path",
	"	//`var naive = \"~path\"",
	"	//`var count = ~\"count",
	"}",
	"",
}, "\n")

func TestSplicesAndQuotesAtRunTime(t *testing.T) {
	got := runWithRuntime(t, "fmt.Printf", map[string]string{"main.go": lists}, nil)
	want := "func f(a int, b string) {}\n" +
		"var statics = []string{\"<p>\", \"say \\\"hi\\\"\", \"C:\\\\temp\"}\n" +
		"var none = []int{}\n" +
		"var quoted = \"C:\\\\temp\"\n" +
		"var naive = \"C:\\temp\"\n" +
		"var count = \"3\"\n"
	if got != want {
		t.Errorf("the generator wrote\n%s\nwant\n%s", got, want)
	}
}

// stringers inserts values with a String method on a value or pointer receiver.
var stringers = strings.Join([]string{
	"package main",
	"",
	"type color int",
	"",
	`func (c color) String() string { return [...]string{"red", "green"}[c] }`,
	"",
	"type tag struct{ name string }",
	"",
	`func (t *tag) String() string { return "<" + t.name + ">" }`,
	"",
	"func main() {",
	"	c := color(1)",
	"	colors := []color{0, 1}",
	`	values := []tag{{"p"}}`,
	`	pointers := []*tag{{"p"}}`,
	"	//`~c ~\"c ~@\"colors",
	"	//`~@\"values ~@\"pointers",
	"}",
	"",
}, "\n")

func TestInterpolationsUseAStringMethod(t *testing.T) {
	got := runWithRuntime(t, "fmt.Printf", map[string]string{"main.go": stringers}, nil)
	if want := "green \"green\" \"red\", \"green\"\n\"{p}\" \"<p>\"\n"; got != want {
		t.Errorf("the generator wrote\n%s\nwant\n%s", got, want)
	}
}

func TestCompileImportsFmtForAQuote(t *testing.T) {
	gen := compile(t, template(`//`+"`"+`x := ~"name`))
	if !strings.Contains(gen, `import "fmt"`) {
		t.Errorf("the generator quotes with fmt and does not import it:\n%s", gen)
	}
}
