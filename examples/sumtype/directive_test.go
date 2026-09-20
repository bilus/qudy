//go:build go1.27

package sumtype_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/sumtype"
)

// source returns a file of package p with the given declarations.
func source(lines ...string) []byte {
	return []byte("package p\n\n" + strings.Join(lines, "\n") + "\n")
}

func TestRenderReadsTheDirective(t *testing.T) {
	type testCase struct {
		description string
		source      []byte
		want        string
	}
	for _, c := range []testCase{
		{
			"as written",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}"),
			"type Shape struct{ v shapeVariant }",
		},
		{
			"as gofmt respells it",
			source("// sumtype: Shape, default=Circle", "type Circle struct{}"),
			"type Shape struct{ v shapeVariant }",
		},
		{
			"below a line of documentation",
			source("// Circle is round.", "//sumtype: Shape, default=Circle", "type Circle struct{}"),
			"type Shape struct{ v shapeVariant }",
		},
		{
			"with spaces around its parts",
			source("//sumtype:  Shape ,  default = Circle", "type Circle struct{}"),
			"and Circle when it is the zero value",
		},
	} {
		t.Run(c.description, func(t *testing.T) {
			out, err := sumtype.Render("p.go", c.source)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), c.want) {
				t.Errorf("rendered\n%s\nwant it to hold\n%s", out, c.want)
			}
		})
	}
}

func TestRenderRejectsADirective(t *testing.T) {
	type testCase struct {
		description string
		source      []byte
		rejects     string
	}
	for _, c := range []testCase{
		{
			"without a name",
			source("//sumtype: default=Circle", "type Circle struct{}"),
			"names the sum type first",
		},
		{
			"without a default",
			source("//sumtype: Shape", "type Circle struct{}"),
			"names the default variant",
		},
		{
			"with an option it does not know",
			source("//sumtype: Shape, default=Circle, json=kind", "type Circle struct{}"),
			"knows one option",
		},
		{
			"whose default is not a variant",
			source("//sumtype: Shape, default=Square", "type Circle struct{}"),
			"default=Square is not a variant of Shape, which has Circle",
		},
		{
			"misplaced, with a blank line before its first variant",
			source("//sumtype: Shape, default=Circle", "", "type Circle struct{}"),
			"p.go:3:1: misplaced directive",
		},
		{
			"misplaced, above a function",
			source("//sumtype: Shape, default=Circle", "func f() {}"),
			"belongs in the doc comment of its first variant",
		},
	} {
		t.Run(c.description, func(t *testing.T) {
			_, err := sumtype.Render("p.go", c.source)
			if err == nil || !strings.Contains(err.Error(), c.rejects) {
				t.Fatalf("Render returned %v, want a rejection naming %q", err, c.rejects)
			}
		})
	}
}
