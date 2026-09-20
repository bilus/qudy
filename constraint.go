package qudy

import (
	"go/build/constraint"
	"strings"
)

// templateTag is the build tag that keeps a template out of a normal build.
const templateTag = "qudy"

// generatorConstraint returns a template's build constraint with the qudy tag negated.
func generatorConstraint(line string) (string, error) {
	expr, err := constraint.Parse(line)
	if err != nil {
		return "", err
	}
	return "//go:build " + negateTemplateTag(expr).String(), nil
}

// isGenerateDirective reports whether a line is a go:generate directive.
func isGenerateDirective(line string) bool {
	return strings.HasPrefix(line, "//go:generate")
}

// isConstraint reports whether a line is a build constraint.
func isConstraint(line string) bool {
	return strings.HasPrefix(line, "//go:build ")
}

// negateTemplateTag returns an expression with every qudy tag negated.
func negateTemplateTag(expr constraint.Expr) constraint.Expr {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		if e.Tag == templateTag {
			return &constraint.NotExpr{X: e}
		}
	case *constraint.NotExpr:
		return &constraint.NotExpr{X: negateTemplateTag(e.X)}
	case *constraint.AndExpr:
		return &constraint.AndExpr{X: negateTemplateTag(e.X), Y: negateTemplateTag(e.Y)}
	case *constraint.OrExpr:
		return &constraint.OrExpr{X: negateTemplateTag(e.X), Y: negateTemplateTag(e.Y)}
	}
	return expr
}
