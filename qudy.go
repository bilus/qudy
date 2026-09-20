// Package qudy compiles a Go template into the generator that writes it.
package qudy

import (
	"fmt"
	"go/format"
	"slices"
	"strconv"
	"strings"
)

// outputMark opens the text of an output line, after the comment slashes.
const outputMark = "`"

// Compile turns a template into the Go source of its generator.
func Compile(filename string, src []byte, emit string, scope Scope) ([]byte, error) {
	t, err := newTemplate(filename, src)
	if err != nil {
		return nil, err
	}
	if t.declared[gensym] {
		return nil, fmt.Errorf("%s: the template declares %s, which is a built-in", filename, gensym)
	}
	var out, run []string
	var runAt int
	declaredAt := map[string]span{}
	opens := t.scopeOpeners()
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		call, names, err := emitCall(emit, run)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filename, runAt, err)
		}
		for _, name := range names {
			if at, ok := declaredAt[name]; ok && at.from <= runAt && runAt <= at.to {
				continue
			}
			if scope.empty() {
				return fmt.Errorf("%s:%d: %s# is a name of the generated program, and the compiler was given no scope", filename, runAt, name)
			}
			if t.declared[name] {
				return fmt.Errorf("%s:%d: the template declares %s, which %s# would shadow", filename, runAt, name, name)
			}
			out = append(out, name+" := "+scope.fresh(name))
			declaredAt[name] = t.blockOf(runAt)
		}
		out = append(out, call)
		run = nil
		return nil
	}
	for i, line := range t.lines {
		text, ok := t.outputs[i+1]
		if !ok {
			if err := flush(); err != nil {
				return nil, err
			}
			if line, err = t.callBuiltins(i+1, line, scope); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", filename, i+1, err)
			}
			out = append(out, line)
			if opens[i+1] && !t.declared[scope.variable] {
				if scope.empty() {
					return nil, fmt.Errorf("%s:%d: the function asks for a fresh name, and the compiler was given no scope", filename, i+1)
				}
				if scope.create == "" {
					return nil, fmt.Errorf("%s:%d: the function asks for a fresh name, and neither the template nor the scope creates %s", filename, i+1, scope.variable)
				}
				out = append(out, scope.decl())
			}
			continue
		}
		if len(run) == 0 {
			runAt = i + 1
		}
		run = append(run, text)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	gen, err := format.Source([]byte(strings.Join(out, "\n")))
	if err != nil {
		return nil, fmt.Errorf("%s: the generator is not Go: %w", filename, err)
	}
	return gen, nil
}

// markText extracts the text of an output comment, in either gofmt spelling.
func markText(comment string) (string, bool) {
	text, ok := strings.CutPrefix(comment, "//")
	if !ok {
		return "", false
	}
	text = strings.TrimPrefix(text, " ")
	return strings.CutPrefix(text, outputMark)
}

// callBuiltins rewrites a line's built-in calls, rightmost first.
func (t *template) callBuiltins(line int, text string, scope Scope) (string, error) {
	calls := t.builtinsOn(line)
	slices.SortFunc(calls, func(a, b builtin) int { return b.from - a.from })
	for _, b := range calls {
		if scope.empty() {
			return "", fmt.Errorf("%s is a name of the generated program, and the compiler was given no scope", b.name)
		}
		text = text[:b.from-1] + scope.call() + text[b.to-1:]
	}
	return text, nil
}

// scopeOpeners finds the opening line of every function body that needs a scope.
func (t *template) scopeOpeners() map[int]bool {
	opens := map[int]bool{}
	for _, b := range t.builtins {
		if f := t.funcOf(b.line); f != (span{}) {
			opens[f.from] = true
		}
	}
	for line, text := range t.outputs {
		// The line's run reports the error.
		_, _, names, err := interpolate(text)
		if err != nil || len(names) == 0 {
			continue
		}
		if f := t.funcOf(line); f != (span{}) {
			opens[f.from] = true
		}
	}
	return opens
}

// splitLines splits a file into lines without their newlines.
func splitLines(src string) []string {
	return strings.Split(src, "\n")
}

// emitCall builds the call that writes a run, with the run's fresh names.
func emitCall(emit string, run []string) (string, []string, error) {
	var text strings.Builder
	var args, names []string
	for _, line := range run {
		format, lineArgs, lineNames, err := interpolate(line)
		if err != nil {
			return "", nil, err
		}
		format, ends := lineEnds(format)
		text.WriteString(format)
		if ends {
			text.WriteByte('\n')
		}
		args = append(args, lineArgs...)
		names = append(names, lineNames...)
	}
	call := emit + "(" + literal(text.String())
	for _, a := range args {
		call += ", " + a
	}
	return call + ")", names, nil
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

// literal quotes the text as a Go string literal, raw where possible.
func literal(text string) string {
	if canBackquote(text) {
		return "`" + text + "`"
	}
	return strconv.Quote(text)
}

// canBackquote reports whether a raw literal holds the text unchanged.
func canBackquote(text string) bool {
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
