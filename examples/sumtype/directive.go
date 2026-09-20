//go:build go1.27

package sumtype

import (
	"fmt"
	"go/token"
	"strings"
)

// directive is a parsed //sumtype: comment.
type directive struct {
	name           string
	defaultVariant string
}

// newDirective returns a directive, or an error for names that are not identifiers.
func newDirective(name, defaultVariant string) (directive, error) {
	if !token.IsIdentifier(name) {
		return directive{}, fmt.Errorf("the directive names the sum type first, as in //sumtype: Shape, default=Circle")
	}
	if !token.IsIdentifier(defaultVariant) {
		return directive{}, fmt.Errorf("the directive names the default variant, as in default=Circle")
	}
	return directive{name: name, defaultVariant: defaultVariant}, nil
}

// parseDirective parses a comment as a directive, in either gofmt spelling.
func parseDirective(comment string) (directive, bool, error) {
	text := strings.TrimPrefix(strings.TrimPrefix(comment, "//"), " ")
	rest, ok := strings.CutPrefix(text, "sumtype:")
	if !ok {
		return directive{}, false, nil
	}
	fields := strings.Split(rest, ",")
	defaultVariant := ""
	for _, option := range fields[1:] {
		key, value, found := strings.Cut(option, "=")
		if !found || strings.TrimSpace(key) != "default" {
			return directive{}, true, fmt.Errorf("the directive knows one option, default=Variant, and got %q", strings.TrimSpace(option))
		}
		defaultVariant = strings.TrimSpace(value)
	}
	d, err := newDirective(strings.TrimSpace(fields[0]), defaultVariant)
	return d, true, err
}
