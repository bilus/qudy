# qudy

qudy is quick and dirty quasiquoting for Go. It compiles a Go template into
the generator that writes it.

A template is a Go file. A comment that opens with a backtick is an output
line: the generator writes that line and does not run it. Every other line is
scaffolding: Go code that runs inside the generator. An interpolation puts the
value of a scaffolding expression into the output.

    qudy -emit g.generate -o writer_gen.go _template/writer.go

## Rules

- A comment of the form `` //`text ``, alone on its line, is an output line. The
  compiler rejects one that follows code. Its text is everything
  after the backtick, and the generator writes a newline after it. gofmt writes
  one space after the slashes when it reindents the line, and both spellings
  are the same output line.
- A comment of the generated program is output text, so it opens with the mark
  too: `` //`// Shape is one of its variants. ``
- Every other line is scaffolding. The compiler copies it to the generator
  unchanged.
- `~expr` and `~{expr}` interpolate a scaffolding expression. `~~` is a literal
  tilde.
- The shorthand `~expr` takes a name and then as many `.name` as follow it. A
  call or an index needs the braces. `~xs[0]` writes the value of `xs` and then
  a literal `[0]`, because a bracket or a paren after a name usually belongs to
  the generated program.
- `name#` is a name of the generated program. Its first use in a block declares
  `name := GENSYM("name")`, and every use writes that name. `##` is a literal
  hash.
- `~//` starts a note to the reader of the template. The compiler drops it,
  together with the spaces before it.
- An output line that ends with a backslash writes no newline, so a loop can
  write the parts of one line. Two backslashes write one backslash.
- A run of consecutive output lines becomes one call to the emitter, with `%s`
  for each interpolation.

## Built-ins

- `GENSYM(want)` is a name of the generated program, seeded with `want`. Use it
  in scaffolding when the scaffolding chooses the seed or declares the variable.
  `name#` stands for it inside an output line.

The compiler declares the scope for those names at the top of each function
that asks for one, unless the template declares the scope itself.

## What a template is not

A template parses as Go, so gofmt formats it and an editor reads it. It does
not compile. Only the output lines use some of the names in its scaffolding,
and to the Go compiler those lines are comments. Keep a template in a directory
that is invisible to the go tool, such as `_template` or `testdata`.

A comment of the template itself cannot open with a backtick. Outside a
function body the compiler rejects such a comment, because an output line
belongs inside one.
