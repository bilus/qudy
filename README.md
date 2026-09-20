# qudy

qudy is a Go library and command for writing Go code generators. Keep loops,
conditions, and helper calls in Go, and write the source you want to produce in
comments next to that logic. qudy compiles those comments into calls to a
printf-style function.

If you have written a generator full of escaped strings, the difference looks
like this. Given `names := []string{"Red", "Green", "Blue"}`, a template can say:

```go
for _, name := range names {
    //`case ~name:
    //`    return "~name"
}
```

Running the resulting generator writes:

```go
case Red:
    return "Red"
case Green:
    return "Green"
case Blue:
    return "Blue"
```

The loop is ordinary Go. The comments describe the output, and `~name` inserts
the current value.

Use it when your generator mixes Go logic with repetitive source code: methods
for a set of types, handlers for a list of routes, or boilerplate derived from a
schema. The template keeps the output visible beside the logic that produces it,
without a separate language for loops and expressions.

There are two steps: **qudy compiles your template into a Go generator; you run
that generator to write the final source.** You supply the data and the logic
that reads it. The generator contains ordinary Go calls and needs no qudy
runtime dependency.

The example below takes an enum's constant names as arguments. After that, read
about interpolation, integration with existing generators, and writing several
files.

## A stringer in one file

Suppose you want `fmt.Sprint(Green)` to print `Green` rather than `1`. In an
existing Go module, save this enum as `color.go`:

```go
// color.go
package colors

type Color int

const (
    Red Color = iota
    Green
    Blue
)
```

The template below generates its `String` method. It takes the package, type,
and constant names from the command line; it does not discover them in Go
source. Create a `stringer` directory and save this as `stringer/stringer.go`:

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
to reduce the chance of clashing with names in the generated code. See
[generated names](#generate-names-with-fewer-collisions) for the limits.

See [examples/stringer](examples/stringer) for a slightly longer version.

## Run the example

Install the command with Go 1.25 or later:

```sh
go install github.com/bilus/qudy/cmd/qudy@latest
```

First, compile the template into a Go program:

```sh
qudy -emit fmt.Printf -o stringer/stringer_gen.go stringer/stringer.go
```

Then run that program to generate the method:

```sh
go run ./stringer colors Color Red Green Blue > color_string.go
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

Run `go test ./...` to check that the package builds with its new method.

## How the template becomes a generator

qudy turns the template's two output lines inside the loop into a call
equivalent to:

```go
fmt.Printf("\tcase %v:\n\t\treturn \"%v\"\n", name, name)
```

The `-emit` flag chooses that function; it defaults to `g.generate`. You can use
an existing generator's method to write to a buffer or file. qudy combines a
**run** of consecutive output lines into one call, uses `%v` for inserted values,
and escapes literal percent signs, which is why the template can say `%d`.
Your emit function must accept a format string and its arguments and interpret
them like `fmt.Printf`.

## Build tags and editor support

A template parses as Go, so `gofmt` can format it and your editor can read it.
It may not build as Go: variables used only in output lines look unused to the
Go compiler. So start a template with `//go:build qudy`, and a normal build
ignores it. qudy writes `//go:build !qudy` into the generator, so the two never
build together. By convention, name the generator after its template, with
`_gen.go` in place of `.go`; the CLI writes to the path supplied with `-o`.

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

## Generate names with fewer collisions

A generator often needs its own local names in the code it produces. Write
`name#` to request a name with a numbered suffix, often called a **gensym**:

```go
//`value, errName# := parse(input)
//`if errName# != nil {
//`    return errName#
//`}
```

The first use of `errName#` in a block declares a variable in the generator,
equivalent to `errName_qd := GENSYM("errName")`. Later uses in that block insert
the same name.

`errName#` never collides with a variable of the template. qudy keeps the
gensym in a generator variable of its own, `errName_qd`, so `errName#` and
`~errName` can stand in one output line and mean different things. For the same
reason a template may not declare a name that ends in `_qd`.

`name#` belongs in output lines. When the template's Go code needs the gensym,
call `GENSYM`, as shown below.

qudy supplies the symbol generator. It declares `qudyGensym`, a small function
value, at the start of each function that uses a gensym, so the generator stays
one self-contained file. Each gensym gets a numbered suffix, so `errName#` comes
out as `errName_qd1`, which is unlikely to collide with a name in your code.

The default generator does not inspect the output's scope or reserve existing
identifiers. A user-defined `errName_qd1` can still collide with its result.
If your generator knows which names are taken, supply a custom symbol generator
that checks them.

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

Have your generator print a txtar archive: a line such as `-- color_string.go --`
starts a new file. Inside a template function, file markers are ordinary output
lines:

```go
//`-- color_string.go --
//`package colors
//`// String methods go here.
//`-- color_parse.go --
//`package colors
//`// Parse functions go here.
```

Pipe the generator's output into `qudy -txtar -o ./generated`. qudy splits the
archive into files under that directory and formats the `.go` files with
`go/format`. For a complete example, run this from the qudy repository:

```sh
go run ./examples/enums -package colors Color=Red,Green,Blue | qudy -txtar -o ./generated
```

See [examples/enums](examples/enums) for a generator that writes both `String`
methods and `Parse` functions. The generated methods expect the enum types and
constants to be defined in the same package.

## Use qudy as a library

Call `qudy.Compile` when you want to compile templates from your own Go program:

```go
generator, err := qudy.Compile("stringer/stringer.go", src, "fmt.Printf")
```

Import `github.com/bilus/qudy` and pass the template as `src []byte`.
`Compile` returns the generator's formatted Go source and an error. It does not
run the generator.

## Further examples

- [stringer](examples/stringer): a complete enum generator and `go generate` integration.
- [enums](examples/enums): several output files from one template.
- [sumtype](examples/sumtype): a larger generator that reads Go source.

The name stands for “quick and dirty quasiquoting”: quote the code you want to
produce, and insert Go values where it varies.
