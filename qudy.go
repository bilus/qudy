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
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		callee := emit
		if runtime {
			callee = outVariable
		}
		call, gensyms, err := emitCall(callee, run)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filename, runAt, err)
		}
		for _, name := range gensyms {
			if in, ok := declaredIn[name]; ok && in.from <= runAt && runAt <= in.to {
				continue
			}
			gen = append(gen, gensymName(name)+" := "+gensymVariable+"("+strconv.Quote(name)+")")
			declaredIn[name] = t.blockOf(runAt)
		}
		gen = append(gen, call)
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
			continue
		}
		if i+1 < t.packageLine && isConstraint(line) {
			if line, err = generatorConstraint(line); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", filename, i+1, err)
			}
		}
		gen = append(gen, line)
		if i+1 == t.packageLine && t.needsFmt(emit) {
			gen = append(gen, `import "fmt"`)
		}
		if bodies[i+1] && !runtime {
			gen = append(gen, gensymDecl)
		}
		if outputs[i+1] && runtime {
			gen = append(gen, outDecl(emit))
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
		_, _, gensyms, err := scanOutputLine(text)
		if err != nil || len(gensyms) == 0 {
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

// emitCall builds the emit call of a run and lists the run's gensyms.
func emitCall(emit string, run []string) (string, []string, error) {
	var text strings.Builder
	var args, gensyms []string
	for _, line := range run {
		lineFormat, lineArgs, lineGensyms, err := scanOutputLine(line)
		if err != nil {
			return "", nil, err
		}
		lineFormat, ends := lineEnds(lineFormat)
		text.WriteString(lineFormat)
		if ends {
			text.WriteByte('\n')
		}
		args = append(args, lineArgs...)
		gensyms = append(gensyms, lineGensyms...)
	}
	call := emit + "(" + stringLiteral(text.String())
	for _, a := range args {
		call += ", " + a
	}
	return call + ")", gensyms, nil
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
