//go:build go1.27

package sumtype

import (
	"fmt"
	"strings"
)

//go:generate go run ../../cmd/qudy -emit g.generate -o write_gen.go write.go

// writer collects the text of a generated file.
type writer struct {
	out strings.Builder
}

// newWriter returns an empty writer.
func newWriter() *writer {
	return &writer{}
}

// generate writes one piece of the generated file.
func (g *writer) generate(format string, args ...any) {
	fmt.Fprintf(&g.out, format, args...)
}
