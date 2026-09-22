package qudy

import (
	"strconv"
	"strings"
)

// lineDirective maps the next line of the generator to a template line.
func lineDirective(filename string, line int) string {
	return "//line " + filename + ":" + strconv.Itoa(line)
}

// inlineDirective maps the rest of a generator line to a template line.
func inlineDirective(filename string, line int) string {
	return "/*line " + filename + ":" + strconv.Itoa(line) + "*/"
}

// isCode reports whether a template line holds more than blanks or a line comment.
func isCode(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && !strings.HasPrefix(trimmed, "//")
}

// statement is one statement of the generator with the template line it stands for.
type statement struct {
	text string
	line int
}

// newStatement returns a statement of the generator for a template line.
func newStatement(text string, line int) statement {
	return statement{text: text, line: line}
}

// argGroup is the arguments of one output line, with that line's number.
type argGroup struct {
	line int
	args []string
}

// newArgGroup returns the arguments that one output line contributes.
func newArgGroup(line int, args []string) argGroup {
	return argGroup{line: line, args: args}
}
