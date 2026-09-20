//go:build go1.27

package sumtype_test

import (
	"os"
	"strings"
	"testing"

	"github.com/bilus/qudy/examples/sumtype"
)

func TestRenderTheShapes(t *testing.T) {
	src, err := os.ReadFile("testdata/shapes/shapes.go")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/shapes/shapes_sumtype.golden")
	if err != nil {
		t.Fatal(err)
	}
	out, err := sumtype.Render("testdata/shapes/shapes.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(want) {
		t.Errorf("rendered\n%s\nwant\n%s", out, want)
	}
}

func TestRenderNamesTheDeclarations(t *testing.T) {
	type testCase struct {
		description string
		source      []byte
		want        string
	}
	for _, c := range []testCase{
		{
			"an exported sum type gets an exported constructor",
			source("//sumtype: Shape, default=Circle", "type Circle struct{}"),
			"func NewShape[V shapeOf](v V) Shape {",
		},
		{
			"an unexported sum type gets an unexported constructor",
			source("//sumtype: shape, default=circle", "type circle struct{}"),
			"func newShape[V shapeOf](v V) shape {",
		},
		{
			"a branch is named after its variant",
			source("//sumtype: shape, default=circle", "type circle struct{}"),
			"onCircle func(circle) R,",
		},
		{
			"the result type avoids a variant named R",
			source("//sumtype: Reading, default=R", "type R float64"),
			"Match[R1 any](\n\tonR func(R) R1,\n) R1 {",
		},
		{
			"the receiver avoids a variant named s",
			source("//sumtype: Reading, default=s", "type s float64"),
			"func (s1 Reading) Match[R any](",
		},
		{
			"the zero value is the default, whatever its kind",
			source("//sumtype: Reading, default=Ohms", "type Ohms float64"),
			"var zero Ohms\n\treturn onOhms(zero)",
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
