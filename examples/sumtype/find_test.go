//go:build go1.27

package sumtype_test

import (
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/sumtype"
)

func TestRenderFindsTheVariants(t *testing.T) {
	type testCase struct {
		description string
		source      []byte
		want        string
	}
	for _, c := range []testCase{
		{
			"on the lines right after the directive",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}", "type Rect struct{}", "type Line struct{}"),
			"\tCircle | Rect | Line\n",
		},
		{
			"up to a blank line",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}", "type Rect struct{}", "", "type Point struct{}"),
			"\tCircle | Rect\n",
		},
		{
			"up to a declaration that is not a type",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}", "var origin = 0", "type Rect struct{}"),
			"\tCircle\n",
		},
		{
			"through a variant's own documentation",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}", "// Rect has corners.", "type Rect struct{}"),
			"\tCircle | Rect\n",
		},
		{
			"in a grouped declaration",
			source("//sumtype: Shape, default=Circle", "type (", "\tCircle struct{}", "", "\tRect struct{}", ")", "type Point struct{}"),
			"\tCircle | Rect\n",
		},
		{
			"up to the next directive",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}", "//sumtype: Unit, default=Metre", "type Metre float64"),
			"\tCircle\n",
		},
	} {
		t.Run(c.description, func(t *testing.T) {
			out, err := sumtype.Render("p.go", c.source)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), c.want) {
				t.Errorf("rendered\n%s\nwant it to hold\n%q", out, c.want)
			}
		})
	}
}

func TestRenderRejectsAVariant(t *testing.T) {
	type testCase struct {
		description string
		source      []byte
		rejects     string
	}
	for _, c := range []testCase{
		{
			"that is an alias",
			source("//sumtype: Shape, default=Circle", "type Circle = struct{}"),
			"p.go:4:6: Circle is an alias",
		},
		{
			"that is an interface",
			source("//sumtype: Shape, default=Circle", "type Circle interface{ Area() float64 }"),
			"Circle is an interface",
		},
		{
			"with type parameters",
			source("//sumtype: Shape, default=Circle", "type Circle[T any] struct{ R T }"),
			"Circle has type parameters",
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

func TestRenderRejectsARedeclaredSumType(t *testing.T) {
	src := source("//sumtype: Shape, default=Circle", "type Circle struct{}", "", "//sumtype: Shape, default=Rect", "type Rect struct{}")
	_, err := sumtype.Render("p.go", src)
	if err == nil || !strings.Contains(err.Error(), "Shape redeclared in this file") {
		t.Fatalf("Render returned %v, want a rejection naming the redeclared sum type", err)
	}
}

func TestRenderOfAFileWithoutASumTypeIsNil(t *testing.T) {
	out, err := sumtype.Render("p.go", source("type Circle struct{}"))
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Errorf("rendered\n%s\nwant nothing", out)
	}
}
