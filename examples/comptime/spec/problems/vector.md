# An array type of computed length

## Problem

A program wants a function that describes a type from a number, as Zig's
`fn Vector(comptime n: usize) type` does:

```go
func vector(n int) ??? // an array of n int64
```

Go has no such function. A type in Go source must be written out, and an
array length must be a constant expression. `[n]int64` with a parameter `n`
does not compile:

```go
func vector(n int) comptime.Type {
	return comptime.TypeFor[[n]int64]() // Go 1.27: invalid array length n
}
```

## Solution

> **Strict comptime, not opportunistic.** A computed type has to be written
> into a generated file before Go can use it, so the source does not build on
> its own. This follows [the strict proposal](../proposal-alternative-strict-comptime.md),
> which is not approved.

A type function returns a `comptime.Type`, built with constructors that
mirror `reflect.ArrayOf`, `reflect.StructOf` and `reflect.PointerTo`:

```go
// vector describes an array of n int64 coordinates.
func vector(n int) comptime.Type {
	return comptime.ArrayOf(n, comptime.TypeFor[int64]())
}
```

- `vector(3)` is the same type as `[3]int64`.
- Code can work on a value of that type through `comptime.Value`, using the
  same methods as `reflect.Value`:

```go
// norm2 returns the sum of squares of v, an array of int64.
func norm2(v comptime.Value) int64 {
	var s int64
	for i := range v.Type().Len() {
		x := v.Index(i).Int()
		s += x * x
	}
	return s
}
```

- `var point = vector(3)` holds a description of a type, not a type. To get a
  name that Go code can use, such as `type point [3]int64`, the type must be
  reified. That is [geo.md](geo.md).

Earlier drafts wrote the parameter as `size comptime.Const[int]`, called as
`vector(comptime.Lit(3))`. The strict proposal writes `vector(n int)`: the
call is evaluated only when every argument is known at build time, and a
`Type` that is not known at build time is an error.

## Evidence

- `reflect.ArrayOf(4, int64)` is the same type as `[4]int64`.
- With the `comptime` package running on `reflect`, `norm2` of a
  `vector(3)` value holding 1, 2 and 3 is 14, and `comptime.As[[3]int64]`
  gives back `[3]int64{1, 2, 3}`.

## Open points

- Computed types in the type positions of ordinary Go code, as in Zig's
  `var p: Vector(3)`, are out of scope: Go needs a type written in the source
  there.
