# Proposal: strict comptime, an alternative design

**Status: proposal, not approved.** This file is an alternative to the design
that [README.md](README.md) and [requirements.md](requirements.md) describe,
"Opportunistic comptime for Go". It changes none of the approved requirements.
It uses the terms of [vocabulary.md](vocabulary.md), and it adds new ones
where it needs them.

## Summary

The approved design keeps a dual-time program: the source runs with or
without `go-comptime`, a `//qudy:comptime` directive asks for evaluation, and
`go-comptime` builds a comptime file through `-overlay`. This proposal is a
second tool beside it:

| | Opportunistic comptime (approved) | Strict comptime (this proposal) |
| --- | --- | --- |
| What marks comptime | a `//qudy:comptime` directive | the types of a `comptime` package |
| Without `go-comptime` | the source builds and runs | the source does not build; the generated code does |
| How the result reaches the build | an overlay of comptime files | generated `.go` files, committed |
| What a dependent package needs | `go-comptime` for the fast version | nothing: plain `go build` |
| What it can do beyond evaluation | nothing | specialize functions on types and constants, and compute types |

Strict comptime is a code generator in the sense of qudy and bennu. Its
source is Go code that works on types as values, close to Zig's `comptime`.
`go generate` runs `go-comptime`, which evaluates that code and writes plain
Go.

## Files and build tags

A package that uses strict comptime has three kinds of file:

| File | Build tag | Written by | Holds |
| --- | --- | --- | --- |
| comptime source, such as `geo.go` | `//go:build comptime` | the programmer | code that uses the `comptime` package |
| generated file, such as `geo_gen.go` | `//go:build !comptime` | `go-comptime` | plain Go; no import of `comptime` |
| any other file, such as `doc.go` | none | the programmer | the `//go:generate go-comptime` line, and ordinary code |

- A normal build, and every dependent package, compiles the generated files.
  The `comptime` package never reaches a binary.
- `go-comptime` reads the comptime source files by name. It does not need the
  build tags; only gopls does (see [gopls](#gopls)).
- The generated files are committed, like the output of qudy.
- `//go:generate` goes in a file without the tag, so that `go generate` finds
  it in a normal build.
- Each generated declaration and statement carries a line directive to its
  source, so compile errors, `go vet` and panics report the comptime source.
  The directives name files by their base names, because the source and the
  generated file share a directory: on Go 1.27 a relative name resolves
  against the directory of the file that holds the directive, and an absolute
  path would put the machine's directories into committed files.

## The `comptime` package

The package mirrors `reflect`, cut down to what `go-comptime` can evaluate or
lower to plain Go. Values that `go-comptime` knows at build time have types of
their own, so the type checker shows where build time ends.

### Types

| Declaration | Meaning |
| --- | --- |
| `type Type` | A description of a Go type, known at build time. |
| `func TypeFor[T any]() Type` | The description of `T`. |
| `func (t Type) Kind() reflect.Kind`, `Elem() Type`, `Len() int` | As in `reflect`. |
| `func (t Type) Fields() iter.Seq[Field]` | The exported fields of a struct, in order. There is no `Field(i int)`. |
| `type Field struct { Name string; Type Type; Tag reflect.StructTag }` | A field. `Fields` yields it, and `StructOf` takes it. |
| `func ArrayOf(n int, elem Type) Type`, `StructOf(fields ...Field) Type`, `PointerTo(t Type) Type`, `SliceOf`, `MapOf` | Builders, one for each kind, as in `reflect` and in Zig's `@Struct` and `@Int`. |
| `func Same[T any](t Type) bool` | Whether the underlying type of `T` is the type that `t` describes, tags included. |

### Values

A `Value` is a run-time value whose type `go-comptime` knows. Its type is
untyped for Go's type checker, because Go cannot name the type of a field
that a `Field` selects.

| Declaration | Lowered to |
| --- | --- |
| `func ValueOf[T any](p *T) Value` | `*p` |
| `func (v Value) Type() Type` | a type known at build time |
| `func (v Value) Field(f Field) Value`, `FieldNamed(name string) Value` | `x.Name`; the field must be known at build time |
| `func (v Value) Index(i int) Value`, `Len() int` | `x[i]`, `len(x)`; `i` may be a run-time value |
| `func (v Value) Int() int64`, `String() string`, `Bool() bool`, `IsNil() bool`, `Elem() Value` | the value of that kind |
| `func (v Value) Set(x Value)`, `SetInt(n int64)` | an assignment |
| `func As[T any](v Value) T`, `func Set[T any](v Value, x T)` | the value as `T`, without boxing; a wrong `T` is an error at build time |
| `func (v Value) Reflect() reflect.Value` | the start of run-time reflection, see [the gate](#the-reflect-gate) |

### Constants and evaluation

| Declaration | Meaning |
| --- | --- |
| `type Const[T any]` with `func Lit[T any](x T) Const[T]` and `func (c Const[T]) Get() T` | A parameter whose argument must be known at build time. `go-comptime` specializes the function once for each distinct argument. |
| `func Eval[T any](f func() T) T` | Run `f` at build time, and write its result as a literal. |
| `func Reify[T any](t Type) struct{}` and `type Reified` | Make `T` the type that `t` describes, see [type functions](#type-functions-and-reify). |

## Rules

### What is known at build time

- `TypeFor[T]()` with a type `T` written in the source, a constant, a `Lit`,
  and the parameters of types `Type`, `Field` and `Const` are known at build
  time.
- A call is evaluated at build time, and replaced by a literal, when every
  argument is known at build time, the function is pure, it ends, and its
  result has a comptime type in the sense of
  [comptime-rules.md](comptime-rules.md). Pure means: no input or output, no
  clock or random numbers, no goroutines or channels, no reads of variables
  that change, and no writes through pointers that outlive the call. A result
  that holds a reference does not leave build time, as R3 says.
- No mark is needed inside such a function. `parseRules(f.Tag.Get("validate"))`
  is evaluated because the tag is known at build time.
- A package-level variable that holds a `Type`, such as
  `var point = vector(comptime.Lit(3))`, is known at build time when it is
  unexported and assigned only in its declaration.

### Specialization

When some arguments are known at build time and some are not, `go-comptime`
specializes the function: it writes one copy for each distinct set of values
known at build time. In each copy:

- a loop over data known at build time, such as `t.Fields()` or an array of
  known length, is unrolled;
- a branch on a value known at build time is folded;
- an operation on a `Value` is lowered by the table above.

`Field(f)` needs `f` at build time, because Go names a field in the source.
`Index(i)` and `MapIndex(k)` need only the type, because Go indexes with
run-time values. A loop over `b.Items` therefore stays a loop, while a loop
over the rules of a tag is unrolled.

### Errors instead of slow paths

Strict comptime has no fallback. `go-comptime` reports an error at the source
line when:

- an argument of type `Const` or `Type` is not known at build time, for
  example `comptime.Lit(userInput)`, or a `Type` taken from a map with a
  run-time key;
- evaluation panics; `panic` plays the part of Zig's `@compileError`;
- `As[T]` names a type other than the value's type.

### The `Reflect` gate

`v.Reflect()` leaves build time on purpose. `go-comptime` writes the value as
`reflect.ValueOf(&x).Elem()` at that point, and everything after it is
ordinary run-time reflection: `Interface()`, `Call`, `MethodByName`, or a call
into a library that takes `any`. The pattern `.Reflect().Interface()` lowers
to `any(x)`. There is no way back from `reflect.Value` to `Value`.
`go-comptime` lists the gates that remain in each function.

## Type functions and `Reify`

A type function returns a `Type`. It may be generic, so that its argument is a
type:

```go
// patch describes a struct with a pointer to each field of T; nil leaves the field unchanged.
func patch[T any]() comptime.Type {
	var fields []comptime.Field
	for f := range comptime.TypeFor[T]().Fields() {
		f.Type = comptime.PointerTo(f.Type)
		fields = append(fields, f)
	}
	return comptime.StructOf(fields...)
}
```

Go cannot compute a type in a type declaration, so a name gets its type in
two lines: a placeholder declaration, and a `Reify` call that says what to
compute.

```go
type configPatch struct{} // computed by the Reify line below

var _ = comptime.Reify[configPatch](patch[config]())
```

- `go-comptime` evaluates the `Reify` calls first. It then type-checks the
  source with each placeholder replaced by its computed type, and reports
  errors at source lines.
- The generated file declares `type configPatch struct { … }` with the
  computed fields and tags. A reified type is a defined type, so it can have
  methods.
- Methods are written by hand in the source on the reified type, and are
  specialized like any function. Computed method sets, such as one method for
  each field, are out of scope; they add declarations, and that is qudy's job.
- A placeholder may embed `comptime.Reified` instead of being empty. The type
  then has the methods of `Value`, so code that reads it through those methods
  type-checks in gopls.
- Struct tags are part of `Field`. They are copied, changed or computed like
  names and types. Tags are part of a struct's type, so `Reify` and `Same`
  compare them, and `go vet` checks them in the generated file.

The name follows Zig: `@Type` used to reify a description of a type into a
type, and Zig 0.16 replaced it with one builtin for each kind.

## gopls

gopls checks a comptime source only when it builds with the comptime tag:

```json
"gopls": { "buildFlags": ["-tags=comptime"] }
```

- Without the setting, gopls reports nothing for a comptime source, not even
  an undefined name.
- With the setting, gopls checks the comptime source and leaves out the
  generated files, through their `!comptime` tag. Without that tag, every
  declaration would be reported as redeclared.
- Uses of computed members, such as `p.Port` on a placeholder, are reported
  as errors. The rest of the file type-checks.
- Other packages in the workspace that import a comptime package see its
  source. Computed members should therefore stay unexported, or be reached
  through methods.
- gopls cannot read a user's overlay file, so stubs that exist only in an
  overlay do not help. With `-overlay` in `GOFLAGS`, gopls drops all
  diagnostics of the package.

## Examples

### A point with a computed array type

```go
// geo.go
//go:build comptime

package geo

import "example.com/geo/comptime"

// vector describes an array of n int64 coordinates.
func vector(n int) comptime.Type {
	return comptime.ArrayOf(n, comptime.TypeFor[int64]())
}

// point is a point in three dimensions.
type point struct{ comptime.Reified }

var _ = comptime.Reify[point](vector(3))

// newPoint returns the point at x, y, z.
func newPoint(x, y, z int64) point {
	var p point
	for i, c := range []int64{x, y, z} {
		p.Index(i).SetInt(c)
	}
	return p
}

// Norm2 returns the square of the point's distance from the origin.
func (p point) Norm2() int64 {
	var s int64
	for i := range p.Len() {
		c := p.Index(i).Int()
		s += c * c
	}
	return s
}
```

`go-comptime` writes:

```go
// geo_gen.go
// Code generated by go-comptime from geo.go. DO NOT EDIT.

//go:build !comptime

package geo

// point is a point in three dimensions.
//
//line geo.go:13:6
type point [3]int64

// newPoint returns the point at x, y, z.
//
//line geo.go:18:1
func newPoint(x, y, z int64) point {
	var p point
//line geo.go:21:3
	p[0], p[1], p[2] = x, y, z
//line geo.go:23:2
	return p
}

// Norm2 returns the square of the point's distance from the origin.
//
//line geo.go:27:1
func (p point) Norm2() int64 {
//line geo.go:31:3
	return p[0]*p[0] + p[1]*p[1] + p[2]*p[2]
}
```

`vector(n)` cannot return `comptime.TypeFor[[n]int64]()`: an array length in
Go must be a constant, and Go 1.27 reports `invalid array length n`.

### A patch with a method

```go
type configPatch struct{} // computed by the Reify line below

var _ = comptime.Reify[configPatch](patch[config]())

// Apply copies each field the patch sets into c.
func (p configPatch) Apply(c config) config {
	cv, pv := comptime.ValueOf(&c), comptime.ValueOf(&p)
	for f := range comptime.TypeFor[config]().Fields() {
		if x := pv.FieldNamed(f.Name); !x.IsNil() {
			cv.Field(f).Set(x.Elem())
		}
	}
	return c
}
```

The generated `Apply` holds one `if p.Name != nil { c.Name = *p.Name }` for
each field, with no reflection.

### Rules from a struct tag

```go
func check(v comptime.Value, validate *validator.Validate) error {
	for f := range v.Type().Fields() {
		for _, r := range parseRules(f.Tag.Get("validate")) {
			x := v.Field(f)
			switch {
			case r.Name == "min" && f.Type.Kind() == reflect.String:
				n, _ := strconv.Atoi(r.Param)
				if utf8.RuneCountInString(x.String()) < n {
					return fmt.Errorf("%s: min=%s", f.Name, r.Param)
				}
			default:
				if err := validate.Var(x.Reflect().Interface(), r.Name+paramSuffix(r)); err != nil {
					return fmt.Errorf("%s: %s", f.Name, r.Name)
				}
			}
		}
	}
	return nil
}
```

For a field `Name string` with the tag `validate:"required,min=2"`, the
specialized code calls the validator for `required` with `any(s.Name)`, and
checks `min=2` with `utf8.RuneCountInString(s.Name) < 2`. A
`*validator.Validate` is never known at build time: it holds functions,
caches and a mutex, and none of them can be written as a literal.

## Not in this proposal

- Computed types in type positions of ordinary Go code, as in Zig's
  `var p: Vector(3)`. Go needs a type in the source text there.
- Computed method sets, and other new declarations. qudy writes those.
- Stub files for gopls, and a gopls proxy.
- Reading files at build time.

## Open questions

1. The name of the build tag: `comptime`, or a name less likely to collide
   with the tags of dependencies, such as `gocomptime`.
2. A flag such as `go-comptime -w` that writes the computed fields into the
   placeholder declaration in the comptime source, as `gofmt -w` rewrites a
   file. gopls would then stop reporting computed members.
3. How `go-comptime` evaluates: by compiling and running a helper program, or
   with an interpreter.
4. A limit on the number of copies that specialization writes for one
   function.
5. Whether strict and opportunistic comptime share the analyzer of
   [comptime-rules.md](comptime-rules.md), and whether one package may use
   both.

## Evidence

These checks ran on Go 1.27 unless a line says otherwise. None used
`go-comptime`, which does not exist yet; each generated file above was
written by hand as the output that `go-comptime` must produce.

- The `comptime` package as described runs on `reflect`: `TypeFor`, `Fields`,
  `Value`, `As`, `Set`, `Const`, `ArrayOf`, `StructOf` and `PointerTo`.
- `reflect.ArrayOf(4, int64)` is the same type as `[4]int64`, and
  `reflect.StructOf` with a field `Data [4]int64` is the same type as
  `struct{ Data [4]int64 }`.
- `patch[config]()` describes the struct with pointer fields and the original
  tags. `patch[int]()` panics with `reflect: NumField of non-struct type int`.
- The geo example builds and passes `go vet` with and without
  `-tags comptime`; its test of the generated code passes.
- Without `//go:build !comptime` in `geo_gen.go`, `gopls check` with the
  comptime tag reports each declaration as redeclared.
- `gopls check` reports nothing for a comptime source without the tag, and
  reports its errors with the tag.
- With `-overlay` in `GOFLAGS`, `gopls check` printed no diagnostics even for
  a real error, and `gopls definition` failed with `stat …: no such file or
  directory`.
- A version of `check` above, with a `gte` rule as well, agrees with
  `validator.Struct` on six inputs, among them a name of one rune and two
  bytes, which `min=2` rejects. That version read fields through methods,
  `f.Name()`, before `Field` became a struct.
- Line directives that name a file relative to the generated file work for
  `go build` on Go 1.23, 1.25 and 1.27. On Go 1.27 the name resolves against
  the directory of the file that holds the directive.
