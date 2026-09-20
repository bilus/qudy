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

## Generate methods for a family of types

Suppose your package defines an `Event` interface and several event types. Each
type needs the same marker method:

```go
type Event interface {
    isEvent()
}

type UserCreated struct{ ID string }
type UserDeleted struct{ ID string }
```

With qudy, a Go loop writes those methods. Save this template as
`_template/events.go`:

```go
package main

import "fmt"

func main() {
    //`package events
    for _, name := range []string{"UserCreated", "UserDeleted"} {
        //`func (~name) isEvent() {}
    }
}
```

The loop runs in the generator. Each comment that starts with a backtick writes
an **output line**, and `~name` inserts the current type name. Running the
generator produces:

```go
package events
func (UserCreated) isEvent() {}
func (UserDeleted) isEvent() {}
```

## Run the example

Install the command with Go 1.24 or later:

```sh
go install github.com/bilus/qudy/cmd/qudy@latest
```

From the directory containing `_template`, compile the template, then run the
resulting generator:

```sh
qudy -emit fmt.Printf -o _template/events_gen.go _template/events.go
go run _template/events_gen.go > events_gen.go
gofmt -w events_gen.go
```

Place `events_gen.go` alongside the `Event`, `UserCreated`, and `UserDeleted`
definitions in package `events`.

There are two steps because qudy produces the **generator's Go source**. Running
that generator produces the code your application uses:

```text
_template/events.go → qudy → _template/events_gen.go → go run → events_gen.go
     template                     generator                   generated code
```

qudy turns the template's output line into a call equivalent to:

```go
fmt.Printf("func (%s) isEvent() {}\n", name)
```

The `-emit` flag chooses that function; it defaults to `g.generate`. You can use
an existing generator's method to write to a buffer or file. qudy combines
consecutive output lines into one call, uses `%s` for inserted values, and escapes
literal percent signs. Your emit function must accept a format string and its
arguments and interpret them like `fmt.Printf`.

A template parses as Go, so `gofmt` can format it and your editor can read it.
It may not build as Go: variables used only in output lines look unused to the
Go compiler. Keep templates in a directory such as `_template` or `testdata`,
which the Go tool ignores when discovering packages.

## Insert values into the output

Use `~name` for a variable and `~p.Type.Text` for a selector. Use braces for
calls, indexes, or other Go expressions. Unbraced names use ASCII Go
identifiers:

```go
//`func (~name) ~method() {}
//`var label = ~{strconv.Quote(label)}
//`var first = ~{values[0]}
```

An unbraced interpolation stops before a bracket or parenthesis.
For example, `~xs[0]` inserts `xs` followed by the literal text `[0]`.
This lets you write expressions in the generated code without braces around
every inserted name.

qudy formats each inserted value with `%s`. Convert other values to strings in
the template when needed, for example with `~{strconv.Itoa(count)}`.

## Control lines and comments

An output line must be a comment on its own line inside a function body.
Both `` //`text `` and the spelling that `gofmt` produces, `` // `text ``, work.
qudy preserves the template's other Go code, apart from the symbol handling
described below, and formats the generator with `go/format`.

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

The first use of `errName#` in a template block declares a variable in the
generator, equivalent to `errName := GENSYM("errName")`. Later uses in that
scope insert the same name. qudy rejects a declaration that would shadow a
variable in the template.

You supply the symbol generator that chooses these names. It must track names
already taken in the generated code and return a fresh name on each call.
Configure it with three comma-separated values:

```sh
qudy -symbols 'symbols,newSymbolGenerator(),gensym' \
    -o writer_gen.go _template/writer.go
```

Here, `symbols` names the variable, `newSymbolGenerator()` creates it, and
`gensym` names its method. qudy adds `symbols := newSymbolGenerator()` at the
start of each function body that needs it, unless the template already declares
`symbols`. You provide the constructor and method in your generator's package.

If the template declares the symbol generator itself, leave the constructor
empty: `-symbols symbols,,gensym`.

Call `GENSYM(want)` directly inside a function body when you compute the desired
name or need to declare the variable yourself:

```go
local := GENSYM(param.Name)
//`var ~local ~param.Type.Text
```

qudy rewrites `GENSYM` to the configured method. A template that uses no gensyms
does not need `-symbols`.

## Use qudy as a library

Call `qudy.Compile` when you want to compile templates from your own Go program:

```go
generator, err := qudy.Compile(
    "_template/events.go",
    src,
    "fmt.Printf",
    qudy.SymbolGenerator{},
)
```

Import `github.com/bilus/qudy` and pass the template as `src []byte`.
`Compile` returns formatted Go source and an error. Use
`qudy.NewSymbolGenerator(variable, create, method)` instead of the zero value
when the template needs gensyms.
