package qudy

import (
	"fmt"
	"strings"
)

// interpolate reads an output line into a format, its arguments and its fresh names.
func interpolate(line string) (string, []string, []string, error) {
	var text strings.Builder
	var args, names []string
	for i := 0; i < len(line); {
		c := line[i]
		if c == '%' {
			text.WriteString("%%")
			i++
			continue
		}
		if c == '#' {
			if !strings.HasPrefix(line[i:], "##") {
				return "", nil, nil, fmt.Errorf("a # marks a name of the generated program and follows one: %s", line[i:])
			}
			text.WriteByte('#')
			i += 2
			continue
		}
		if isNameStart(c) {
			n := nameLen(line[i:])
			if i+n < len(line) && line[i+n] == '#' && !strings.HasPrefix(line[i+n:], "##") {
				args = append(args, line[i:i+n])
				names = append(names, line[i:i+n])
				text.WriteString("%s")
				i += n + 1
				continue
			}
			text.WriteString(line[i : i+n])
			i += n
			continue
		}
		if c != '~' {
			text.WriteByte(c)
			i++
			continue
		}
		rest := line[i+1:]
		switch {
		case strings.HasPrefix(rest, "//"):
			return strings.TrimRight(text.String(), " \t"), args, names, nil
		case strings.HasPrefix(rest, "~"):
			text.WriteByte('~')
			i += 2
		case strings.HasPrefix(rest, "{"):
			n, err := bracedLen(rest)
			if err != nil {
				return "", nil, nil, err
			}
			args = append(args, strings.TrimSpace(rest[1:n-1]))
			text.WriteString("%s")
			i += 1 + n
		default:
			n, err := exprLen(rest)
			if err != nil {
				return "", nil, nil, err
			}
			args = append(args, rest[:n])
			text.WriteString("%s")
			i += 1 + n
		}
	}
	return text.String(), args, names, nil
}

// bracedLen measures a braced interpolation, skipping braces inside quotes.
func bracedLen(src string) (int, error) {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'', '`':
			n, ok := quotedLen(src[i:])
			if !ok {
				return 0, fmt.Errorf("an interpolation holds a quote that does not close: %s", src)
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
	return 0, fmt.Errorf("an interpolation opens with ~{ and does not close: %s", src)
}

// quotedLen measures the quoted text at the start of src.
func quotedLen(src string) (int, bool) {
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

// exprLen measures the name path at the start of src.
func exprLen(src string) (int, error) {
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

// isNameStart reports whether a byte can open a name.
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
