// Package qudy compiles a template into the Go source of its generator.
package qudy

import (
	"fmt"
	"go/format"
	"strconv"
	"strings"
)

// backtick is the character that opens the text of an output line.
const backtick = "`"

// Compile turns a template into the Go source of its generator.
func Compile(filename string, src []byte, emit string) ([]byte, error) {
	t, err := newTemplate(filename, src)
	if err != nil {
		return nil, err
	}
	return t.compile(emit, false)
}

// CompileWithRuntime returns a template's generator and the runtime file of its package.
func CompileWithRuntime(filename string, src []byte, emit string) (generator, runtime []byte, err error) {
	t, err := newTemplate(filename, src)
	if err != nil {
		return nil, nil, err
	}
	if generator, err = t.compile(emit, true); err != nil {
		return nil, nil, err
	}
	if runtime, err = runtimeSource(t.pkg); err != nil {
		return nil, nil, fmt.Errorf("%s: the runtime file is not Go: %w", filename, err)
	}
	return generator, runtime, nil
}

// compile writes the generator, against the runtime file or with everything inline.
func (t *template) compile(emit string, runtime bool) ([]byte, error) {
	filename := t.filename
	var err error
	var gen, run []string
	var runAt int
	declaredIn := map[string]span{}
	for _, line := range t.gensymCalls {
		if t.funcBodyOf(line) == (span{}) && !runtime && !t.packageGensym {
			return nil, fmt.Errorf("%s:%d: a %s call outside a function body needs -runtime, or a %s of the package", filename, line, gensymVariable, gensymVariable)
		}
	}
	bodies := t.gensymBodies()
	outputs := t.outputBodies()
	// synced holds while the copied template lines keep their own line numbers.
	synced := false
	insert := func(lines ...string) {
		gen = append(gen, lines...)
		synced = false
	}
	copyLine := func(number int, line string) {
		if !synced && isCode(line) {
			gen = append(gen, lineDirective(filename, number))
			synced = true
		}
		gen = append(gen, line)
	}
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		callee := emit
		if runtime {
			callee = outVariable
		}
		statements, gensyms, err := emitStatements(callee, filename, run, runAt)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filename, runAt, err)
		}
		for _, name := range gensyms {
			if in, ok := declaredIn[name]; ok && in.from <= runAt && runAt <= in.to {
				continue
			}
			insert(gensymName(name) + " := " + gensymVariable + "(" + strconv.Quote(name) + ")")
			declaredIn[name] = t.blockOf(runAt)
		}
		for _, s := range statements {
			insert(lineDirective(filename, s.line), s.text)
		}
		run = nil
		return nil
	}
	for i, line := range t.lines {
		text, ok := t.outputLines[i+1]
		if ok {
			if len(run) == 0 {
				runAt = i + 1
			}
			run = append(run, text)
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		if isGenerateDirective(line) {
			// The directive compiles the template, so it stays out of the generator.
			synced = false
			continue
		}
		if i+1 < t.packageLine && isConstraint(line) {
			if line, err = generatorConstraint(line); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", filename, i+1, err)
			}
		}
		copyLine(i+1, line)
		if i+1 == t.packageLine && t.needsFmt(emit) {
			insert(`import "fmt"`)
		}
		if bodies[i+1] && !runtime {
			insert(gensymDecl)
		}
		if outputs[i+1] && runtime {
			insert(outDecl(emit))
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	formatted, err := format.Source([]byte(strings.Join(gen, "\n")))
	if err != nil {
		return nil, fmt.Errorf("%s: the generator is not Go: %w", filename, err)
	}
	return formatted, nil
}

// outputText returns the text after the backtick, in either gofmt spelling.
func outputText(comment string) (string, bool) {
	text, ok := strings.CutPrefix(comment, "//")
	if !ok {
		return "", false
	}
	text = strings.TrimPrefix(text, " ")
	return strings.CutPrefix(text, backtick)
}

// gensymBodies returns the opening lines of function bodies that need the built-in symbol generator.
func (t *template) gensymBodies() map[int]bool {
	bodies := map[int]bool{}
	need := func(line int) {
		if body := t.outerFuncBodyOf(line); !t.ownGensym[body] {
			bodies[body.from] = true
		}
	}
	for _, line := range t.gensymCalls {
		need(line)
	}
	for line, text := range t.outputLines {
		// The line's run reports the error.
		scanned, err := scanOutputLine(text)
		if err != nil || len(scanned.gensyms) == 0 {
			continue
		}
		need(line)
	}
	return bodies
}

// splitLines splits a file into lines without their newlines.
func splitLines(src string) []string {
	return strings.Split(src, "\n")
}

// emitStatements builds the statements that write a run, and lists the run's gensyms.
func emitStatements(emit, filename string, run []string, first int) ([]statement, []string, error) {
	var statements []statement
	var gensyms []string
	var text strings.Builder
	var groups []argGroup
	start, last := 0, 0
	flush := func() {
		if text.Len() == 0 {
			return
		}
		literal := stringLiteral(text.String())
		call, line := emit+"("+literal, start
		// One output line maps its arguments to itself; a run marks each argument group.
		if last == start {
			line = start + 1 - (strings.Count(literal, "\n") + 1)
		}
		for _, g := range groups {
			for i, a := range g.args {
				if i == 0 && last > start {
					call += ", " + inlineDirective(filename, g.line) + " " + a
					continue
				}
				call += ", " + a
			}
		}
		statements = append(statements, newStatement(call+")", line))
		text.Reset()
		groups = nil
	}
	for j, line := range run {
		number := first + j
		scanned, err := scanOutputLine(line)
		if err != nil {
			return nil, nil, err
		}
		gensyms = append(gensyms, scanned.gensyms...)
		for i, p := range scanned.parts {
			if p.splice != "" {
				flush()
				statements = append(statements, newStatement(spliceLoop(emit, p), number))
				continue
			}
			format := p.format
			if i == len(scanned.parts)-1 {
				ends := false
				if format, ends = lineEnds(format); ends {
					format += "\n"
				}
			}
			if text.Len() == 0 {
				start = number
			}
			last = number
			text.WriteString(format)
			if len(p.args) > 0 {
				groups = append(groups, newArgGroup(number, p.args))
			}
		}
	}
	flush()
	return statements, gensyms, nil
}

// spliceLoop writes the elements of a slice with a comma between them.
func spliceLoop(emit string, p part) string {
	element := emit + "(" + strconv.Quote(verb) + ", qudyX)"
	if p.quote {
		element = emit + "(" + strconv.Quote(quoteVerb) + ", " + quoted("qudyX") + ")"
	}
	return "for qudyI, qudyX := range " + p.splice + " {\nif qudyI > 0 {\n" + emit + `(", ")` + "\n}\n" + element + "\n}"
}

// lineEnds strips a trailing backslash and reports whether the line writes a newline.
func lineEnds(text string) (string, bool) {
	if strings.HasSuffix(text, `\\`) {
		return text[:len(text)-1], true
	}
	if strings.HasSuffix(text, `\`) {
		return text[:len(text)-1], false
	}
	return text, true
}

// stringLiteral returns the text as a Go string literal, raw where possible.
func stringLiteral(text string) string {
	if canBeRaw(text) {
		return "`" + text + "`"
	}
	return strconv.Quote(text)
}

// canBeRaw reports whether the text fits a raw string literal.
func canBeRaw(text string) bool {
	for _, r := range text {
		if r == '`' || r == '\r' {
			return false
		}
		if r < ' ' && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
