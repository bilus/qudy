package qudy

import (
	"fmt"
	"strings"
)

// scanOutputLine splits an output line into a format, its arguments and its gensyms.
func scanOutputLine(line string) (string, []string, []string, error) {
	var format strings.Builder
	var args, gensyms []string
	for i := 0; i < len(line); {
		c := line[i]
		if c == '%' {
			format.WriteString("%%")
			i++
			continue
		}
		if c == '#' {
			if !strings.HasPrefix(line[i:], "##") {
				return "", nil, nil, fmt.Errorf("a # follows the name of a gensym, and ## writes a hash: %s", line[i:])
			}
			format.WriteByte('#')
			i += 2
			continue
		}
		if isNameStart(c) {
			n := nameLen(line[i:])
			if i+n < len(line) && line[i+n] == '#' && !strings.HasPrefix(line[i+n:], "##") {
				args = append(args, line[i:i+n])
				gensyms = append(gensyms, line[i:i+n])
				format.WriteString("%s")
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
		switch {
		case strings.HasPrefix(rest, "//"):
			return strings.TrimRight(format.String(), " \t"), args, gensyms, nil
		case strings.HasPrefix(rest, "~"):
			format.WriteByte('~')
			i += 2
		case strings.HasPrefix(rest, "{"):
			n, err := bracedLen(rest)
			if err != nil {
				return "", nil, nil, err
			}
			args = append(args, strings.TrimSpace(rest[1:n-1]))
			format.WriteString("%s")
			i += 1 + n
		default:
			n, err := selectorLen(rest)
			if err != nil {
				return "", nil, nil, err
			}
			args = append(args, rest[:n])
			format.WriteString("%s")
			i += 1 + n
		}
	}
	return format.String(), args, gensyms, nil
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
