//go:build go1.27

package sumtype

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// sumType is one sum type and its variants, in source order.
type sumType struct {
	name           string
	defaultVariant string
	variants       []string
}

// newSumType returns a sum type, and rejects a default variant outside its variants.
func newSumType(d directive, variants []string) (sumType, error) {
	if !slices.Contains(variants, d.defaultVariant) {
		return sumType{}, fmt.Errorf("default=%s is not a variant of %s, which has %s", d.defaultVariant, d.name, strings.Join(variants, ", "))
	}
	return sumType{name: d.name, defaultVariant: d.defaultVariant, variants: variants}, nil
}

// variantInterface names the private interface of the variants.
func (s sumType) variantInterface() string {
	return lowerFirst(s.name) + "Variant"
}

// markerMethod names the method that marks a variant.
func (s sumType) markerMethod() string {
	return "is" + upperFirst(s.name)
}

// constraint names the type constraint that admits the variants.
func (s sumType) constraint() string {
	return lowerFirst(s.name) + "Of"
}

// constructor names the function that wraps a variant, exported with the sum type.
func (s sumType) constructor() string {
	if s.name == upperFirst(s.name) {
		return "New" + s.name
	}
	return "new" + upperFirst(s.name)
}

// union joins the variants into a type union.
func (s sumType) union() string {
	return strings.Join(s.variants, " | ")
}

// branch names the Match parameter for a variant.
func branch(variant string) string {
	return "on" + upperFirst(variant)
}

// upperFirst returns a name with its first letter in upper case.
func upperFirst(name string) string {
	r, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(r)) + name[n:]
}

// lowerFirst returns a name with its first letter in lower case.
func lowerFirst(name string) string {
	r, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[n:]
}
