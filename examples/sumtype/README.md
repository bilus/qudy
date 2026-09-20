# sumtype

sumtype reads a package and writes the boilerplate for its sum types.

    go run ./examples/sumtype/cmd/sumtype ./path/to/package

A directive in the doc comment of a type declaration names the sum type and
its default variant. The type declarations on the lines right after the
directive are its variants.

    //sumtype: Shape, default=Circle
    type Circle struct{ R float64 }
    type Rect struct{ W, H float64 }

For `shapes.go` the tool writes `shapes_sumtype.go` beside it, with every sum
type from that file:

    type Shape struct{ v shapeVariant }

    func NewShape[V shapeOf](v V) Shape
    func (s Shape) Match[R any](onCircle func(Circle) R, onRect func(Rect) R) R

`Match` is a generic method, so the generated package needs Go 1.27. So does
this example: it formats its output with `go/format`, and that parser accepts a
generic method from Go 1.27 on. Every file here carries `//go:build go1.27`, so
an older toolchain skips the example and builds the rest of qudy.

## Terms

- **directive**: a `//sumtype:` comment in the doc comment of a type
  declaration. A **misplaced directive** is one anywhere else.
- **variant**: a type that belongs to a sum type.
- **default variant**: the variant whose zero value the zero sum type stands
  for.
- **variant interface**: the private interface of the variants, such as
  `shapeVariant`.
- **marker method**: the unexported method of the variant interface, such as
  `isShape`.
- **branch**: the `Match` parameter for one variant, such as `onCircle`.
- **source file**, **generated file**: the file with the directive, and the
  `_sumtype.go` file beside it.

## Rules

- The directive is `//sumtype: Name, default=Variant`. gofmt respells it as
  `// sumtype:`, and the tool reads both.
- The directive sits in the doc comment of its first variant. A blank line
  between the two makes it a misplaced directive, as does any declaration
  below it other than a type. A misplaced directive is an error.
- The variants are the type declarations on consecutive lines after the
  directive. A blank line, another directive, or a declaration that is not a
  type ends them. A variant may carry its own doc comment.
- A grouped declaration, `type ( ... )`, under a directive gives the group's
  types as the variants.
- A variant is a defined, concrete type without type parameters. An alias, an
  interface or a generic type is an error.
- `default` is required and names the default variant. The zero value of the
  sum type behaves as the zero value of that variant, so `Match` has a branch
  for every value, without a panic.
- An exported sum type gets `NewName`. An unexported one gets `newName`.
- `NewName` accepts the variant types only. A pointer to a variant does not
  compile.
- The receiver `s` and the result type `R` of `Match` take a number when a
  variant already has that name.
- A sum type redeclared in one source file is an error.
- The tool skips test files and generated files.

## How it is built

`write.go` is a qudy template. Its first line is `//go:build qudy`, so a normal
build ignores it, and `go generate` compiles it into `write_gen.go`. The
directive is in `writer.go`, which is always built.

The template declares its own `qudyGensym`, a symbol generator that starts with
the variant names. The gensyms `s#` and `R#` therefore keep their plain names
unless a variant has the same one, which matters because both appear in the
public signature of `Match`.
