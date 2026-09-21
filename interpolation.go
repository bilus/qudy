package qudy

import (
	"fmt"
	"strings"
)

// verb formats every interpolation and gensym in an emit call.
const verb = "%v"

// quoteVerb formats a quoted interpolation, given the text of its value.
const quoteVerb = "%q"

// part is a piece of an output line, either text or a splice.
type part struct {
	format string
	args   []string
	splice string
	quote  bool
}

// newText returns a piece of text and the arguments of its verbs.
func newText(format string, args []string) part {
	return part{format: format, args: args}
}

// newSplice returns the splice of a slice, with its elements quoted or not.
func newSplice(slice string, quote bool) part {
	return part{splice: slice, quote: quote}
}

// outputLine is a scanned output line, which always ends with a piece of text.
type outputLine struct {
	parts   []part
	gensyms []string
	quotes  bool
}

// newOutputLine returns a scanned line from its parts, gensyms and use of quotes.
func newOutputLine(parts []part, gensyms []string, quotes bool) outputLine {
	return outputLine{parts: parts, gensyms: gensyms, quotes: quotes}
}

// quoted returns the argument of a quoted interpolation.
func quoted(expr string) string {
	return "fmt.Sprint(" + expr + ")"
}

// scanOutputLine splits an output line into its parts and lists its gensyms.
func scanOutputLine(line string) (outputLine, error) {
	var parts []part
	var format strings.Builder
	var args, gensyms []string
	quotes := false
	text := func(trim bool) {
		f := format.String()
		if trim {
			f = strings.TrimRight(f, " \t")
		}
		parts = append(parts, newText(f, args))
		format.Reset()
		args = nil
	}
	for i := 0; i < len(line); {
		c := line[i]
		if c == '%' {
			format.WriteString("%%")
			i++
			continue
		}
		if c == '#' {
			if !strings.HasPrefix(line[i:], "##") {
				return outputLine{}, fmt.Errorf("a # follows the name of a gensym, and ## writes a hash: %s", line[i:])
			}
			format.WriteByte('#')
			i += 2
			continue
		}
		if isNameStart(c) {
			n := nameLen(line[i:])
			if i+n < len(line) && line[i+n] == '#' && !strings.HasPrefix(line[i+n:], "##") {
				args = append(args, gensymName(line[i:i+n]))
				gensyms = append(gensyms, line[i:i+n])
				format.WriteString(verb)
				i += n + 1
				continue
			}
			format.WriteString(line[i : i+n])
			i += n
			continue
		}
		if c != '~' {
			format.WriteByte(c)
			i++
			continue
		}
		rest := line[i+1:]
		if strings.HasPrefix(rest, "//") {
			text(true)
			return newOutputLine(parts, gensyms, quotes), nil
		}
		if strings.HasPrefix(rest, "~") {
			format.WriteByte('~')
			i += 2
			continue
		}
		splice := strings.HasPrefix(rest, "@")
		rest = strings.TrimPrefix(rest, "@")
		quote := strings.HasPrefix(rest, `"`)
		rest = strings.TrimPrefix(rest, `"`)
		expr, n, err := operand(rest)
		if err != nil {
			return outputLine{}, err
		}
		i += len(line[i+1:]) - len(rest) + 1 + n
		quotes = quotes || quote
		switch {
		case splice:
			text(false)
			parts = append(parts, newSplice(expr, quote))
		case quote:
			args = append(args, quoted(expr))
			format.WriteString(quoteVerb)
		default:
			args = append(args, expr)
			format.WriteString(verb)
		}
	}
	text(false)
	return newOutputLine(parts, gensyms, quotes), nil
}

// operand returns the expression of an interpolation and its length in src.
func operand(src string) (string, int, error) {
	if strings.HasPrefix(src, "{") {
		n, err := bracedLen(src)
		if err != nil {
			return "", 0, err
		}
		return strings.TrimSpace(src[1 : n-1]), n, nil
	}
	n, err := selectorLen(src)
	if err != nil {
		return "", 0, err
	}
	return src[:n], n, nil
}

// bracedLen measures a braced interpolation and skips braces inside string and rune literals.
func bracedLen(src string) (int, error) {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'', '`':
			n, ok := literalLen(src[i:])
			if !ok {
				return 0, fmt.Errorf("a braced interpolation holds a string or rune literal without its closing quote: %s", src)
			}
			i += n - 1
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("a braced interpolation has no closing brace: %s", src)
}

// literalLen measures the string or rune literal at the start of src.
func literalLen(src string) (int, bool) {
	quote := src[0]
	for i := 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			if quote != '`' {
				i++
			}
		case quote:
			return i + 1, true
		}
	}
	return 0, false
}

// selectorLen measures the name and its selectors at the start of src.
func selectorLen(src string) (int, error) {
	if len(src) == 0 || !isNameStart(src[0]) {
		return 0, fmt.Errorf("an interpolation starts with a name: ~%s", src)
	}
	end := nameLen(src)
	for end < len(src) && src[end] == '.' {
		n := nameLen(src[end+1:])
		if n == 0 {
			break
		}
		end += 1 + n
	}
	return end, nil
}

// isNameStart reports whether a byte can start a name.
func isNameStart(c byte) bool {
	return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// nameLen measures the name at the start of src.
func nameLen(src string) int {
	n := 0
	for n < len(src) && (isNameStart(src[n]) || '0' <= src[n] && src[n] <= '9') {
		n++
	}
	return n
}
