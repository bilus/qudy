# A variadic `...any` function

## Problem

A function takes any number of arguments of any type, and inspects each one
with reflection:

```go
// foo writes ints as numbers and strings quoted, separated by spaces.
func foo(args ...any) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		v := reflect.ValueOf(a)
		switch v.Kind() {
		case reflect.Int:
			b.WriteString(strconv.FormatInt(v.Int(), 10))
		case reflect.String:
			b.WriteString(strconv.Quote(v.String()))
		default:
			b.WriteString(fmt.Sprint(a))
		}
	}
	return b.String()
}
```

It is called as `foo(x, y)` with two `int` variables and as `foo(s)` with a
`string`. The values are known only at run time. What the compiler knows at
each call is its shape: how many arguments there are and what type each one
has. Yet every call boxes each argument into `any`, allocates the slice, and
asks reflection again for the kind of each argument.

The goal: one fixed-parameter copy of `foo` for each shape, with the callers
unchanged and the function still usable without `go-comptime`.

## Solution

> **Opportunistic, but beyond the first version.** The solution needs
> specialization, which R1 to R18 do not cover. It is a design sketch: the
> `comptime.ValueOf` form below has not been written or run.

The signature stays `args ...any`, so that callers in any package compile with
or without the tool. The function marks the build-time part where it reflects,
by calling `comptime.ValueOf` instead of `reflect.ValueOf`:

```go
		v := comptime.ValueOf(a) // was reflect.ValueOf
		switch v.Kind() {        // depends only on the type: folded per shape
		case comptime.Int:
			b.WriteString(strconv.FormatInt(v.Int(), 10)) // reads the value: becomes int64(a0)
```

- The arguments are not `Const` values: `Const` promises that the value is
  known at build time, and `foo(x, y)` would be rejected. They are not
  `comptime.Value` parameters either: a caller could not pass a plain `int`.
- Methods of `comptime.Value` that depend only on the type, such as `Type`,
  `Kind` and `NumField`, may be folded for each shape. Methods that read the
  value, such as `Int` and `String`, become plain Go on the parameter.
- Without the tool, `comptime.ValueOf` runs on `reflect`.

For each call whose shape is known, the tool unrolls the loop (the length is
known), folds `Kind()`, lowers `Int()` and `String()`, and rewrites the call:

```go
func foo_int_int(a0, a1 int) string {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(int64(a0), 10))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatInt(int64(a1), 10))
	return b.String()
}

func foo_string(a0 string) string {
	var b strings.Builder
	b.WriteString(strconv.Quote(a0))
	return b.String()
}
```

A call keeps using plain `foo` when its shape is unknown:
- an argument whose static type is an interface, as in `foo(err)`;
- a spread slice, as in `foo(xs...)`.

## Why there is no strict version

A strict version would turn every unknown shape into a build error. That loses
the fallback the problem depends on.

## Evidence

The reflection version and the hand-written copies return the same strings.
Measured with `-benchmem`, with values above 255 so that boxing allocates:

| call | reflection | specialized |
| --- | --- | --- |
| two ints | 117 ns, 6 allocs | 92 ns, 4 allocs |
| one string | 71 ns, 3 allocs | 53 ns, 2 allocs |

The gain is about 25%, because `strings.Builder` costs more than the
reflection here. A function that does more reflection for each argument would
gain more.

## Open points

- A limit on the number of copies: a function called with many shapes, such as
  `fmt.Println`, would get one copy for each.
- Whether specialization needs an opt-in directive on `foo`.
- Which `comptime.Value` methods the tool can fold, and which it lowers.
