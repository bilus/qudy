# qudy

qudy helps you write Go code generators. You write loops, conditions, and helper
calls in ordinary Go, then put the code you want to generate in special comments.
qudy turns those comments into calls that write the output.

Use it when your generator mixes Go logic with repetitive source code: methods
for a set of types, handlers for a list of routes, or boilerplate derived from a
schema. The template keeps the output visible beside the logic that produces it,
without a separate language for loops and expressions.

The name stands for “quick and dirty quasiquoting”: quote the code you want to
produce, and insert Go values where it varies.

## A stringer in one file

Suppose your package defines an enum, and you want the type to have a `String` method, printing the names of the constants. TODO

```go
type Color int

const (
    Red Color = iota
    Green
    Blue
)
```

This template is the whole generator. Save it as `stringer/stringer.go`:

```go
//go:build qudy

package main

import (
    "fmt"
    "os"
)

func main() {
    pkg, typ, names := os.Args[1], os.Args[2], os.Args[3:]
    //`package ~pkg
    //`
    //`import "fmt"
    //`
    //`func (v# ~typ) String() string {
    //`	switch v# {
    for _, name := range names {
        //`	case ~name:
        //`		return "~name"
    }
    //`	}
    //`	return fmt.Sprintf("~typ(%d)", v#)
    //`}
}
```

The Go code runs in the generator. Each comment that starts with a backtick
writes an **output line**, `~name` inserts a Go value, and the loop writes one
`case` for each constant. `v#` writes a generated name, something like `v_qd1`,
to avoid clashing with existing variables in the scope.

See [examples/stringer](examples/stringer) for a slightly longer version.

## Run the example

Install the command with Go 1.25 or later:

```sh
go install github.com/bilus/qudy/cmd/qudy@latest
```

Compile the template, then run the generator:

```sh
qudy -emit fmt.Printf -o stringer/stringer_gen.go stringer/stringer.go
```

`color_string.go` now holds:

```go
package colors

import "fmt"

func (v_qd1 Color) String() string {
	switch v_qd1 {
	case Red:
		return "Red"
	case Green:
		return "Green"
	case Blue:
		return "Blue"
	}
	return fmt.Sprintf("Color(%d)", v_qd1)
}
```

To run the generated generator:

``` text
go run ./stringer colors Color Red Green Blue > color_string.go
```

TODO: Review the rest


qudy turns the template's two output lines inside the loop into a call
equivalent to:

```go
fmt.Printf("\tcase %v:\n\t\treturn \"%v\"\n", name, name)
```

The `-emit` flag chooses that function; it defaults to `g.generate`. You can use
an existing generator's method to write to a buffer or file. qudy combines a
**run** of consecutive output lines into one call, uses `%v` for inserted values, and escapes
literal percent signs, which is why the template can say `%d`. Your emit function must accept a format string and its
arguments and interpret them like `fmt.Printf`.

A template parses as Go, so `gofmt` can format it and your editor can read it.
It may not build as Go: variables used only in output lines look unused to the
Go compiler. So start a template with `//go:build qudy`, and a normal build
ignores it. qudy writes `//go:build !qudy` into the generator, so the two never
build together. Name the generator after its template, with `_gen.go` in place
of `.go`: `stringer.go` compiles to `stringer_gen.go`.

To get completion and navigation inside a template, give your editor the tag,
for example `-tags=qudy` in the `buildFlags` of gopls. It then loads the
templates in place of the generators, and it reports a name that only output
lines use as unused.

The quick start runs qudy by hand. To run it with `go generate`, see
[examples/stringer](examples/stringer).

## Insert values into the output

`~name` is an **interpolation**: qudy inserts the value of `name` there. Use
`~name` for a variable and `~p.Type.Text` for a selector. Use a **braced
interpolation**, `~{expr}`, for calls, indexes, or other Go expressions. A name
without braces is an ASCII Go identifier:

```go
//`func (~name) ~method() {}
//`var label = ~{strconv.Quote(label)}
//`var first = ~{values[0]}
```

An interpolation without braces stops before a bracket or parenthesis.
For example, `~xs[0]` inserts `xs` followed by the literal text `[0]`.
This lets you write expressions in the generated code without braces around
every inserted name.

qudy formats each inserted value with `%v`, so a number works as well as a
string: `~count` needs no conversion.

## Output lines and comments

An output line must be a comment on its own line inside a function body.
Both `` //`text `` and the spelling that `gofmt` produces, `` // `text ``, work.
qudy copies the template's other Go code into the generator, with three
exceptions: it negates the `qudy` build tag, it leaves out `go:generate` lines,
and it handles gensyms as described below. It formats the generator with
`go/format`.

Each output line writes a trailing newline. End it with a backslash to continue
on the same line, for example when a loop writes an argument list:

```go
//`handle(ctx\
for _, arg := range args {
    //`, ~arg\
}
//`)
```

Two trailing backslashes write one backslash and keep the newline.

To put a comment in the generated code, include it in an output line:

```go
//`// Event represents a change to a user.
```

To annotate only the template, use an ordinary Go comment or `~//` inside an
output line. qudy drops `~//`, the rest of that line, and the spaces before it:

```go
//`} ~// close the generated handler
```

Write `~~` for a literal tilde and `##` for a literal hash. Reserve comments that
open with a backtick for output lines; qudy rejects them outside function bodies.

## Generate names without collisions

A generator sometimes needs a local name that won't collide with names already
used in the output. Write `name#` to request such a name, often called a
**gensym**:

```go
//`value, errName# := parse(input)
//`if errName# != nil {
//`    return errName#
//`}
```

The first use of `errName#` in a block declares a variable in the generator,
equivalent to `errName := GENSYM("errName")`. Later uses in that block insert
the same name. qudy rejects a declaration that would shadow a
variable in the template.

qudy supplies the symbol generator. It declares `qudyGensym`, a small function
value, at the start of each function that uses a gensym, so the generator stays
one self-contained file. Each gensym gets a numbered suffix, so `errName#` comes
out as `errName_qd1`, which is unlikely to collide with a name in your code.

To choose the names yourself, declare `qudyGensym` in the template, as a local
variable, a parameter, or a package-level variable. It is a
`func(want string) string`. qudy then uses yours and declares none:

```go
qudyGensym := newSeededGensym(taken)
```

A function literal shares the symbol generator of the function around it.

Call `GENSYM(want)` directly inside a function body when you compute the desired
name or need to declare the variable yourself:

```go
local := GENSYM(param.Name)
//`var ~local ~param.Type.Text
```

qudy rewrites `GENSYM` to `qudyGensym`.

## Generate several files

TODO: describe `qudy -txtar`, show a trivial example, and refer to
`examples/enums`.

## Use qudy as a library

Call `qudy.Compile` when you want to compile templates from your own Go program:

```go
generator, err := qudy.Compile("stringer/stringer.go", src, "fmt.Printf")
```

Import `github.com/bilus/qudy` and pass the template as `src []byte`.
`Compile` returns the generator's formatted Go source and an error.
