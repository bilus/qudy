# A struct type derived from another

## Problem

A program has a configuration struct and wants a patch type for it. The patch
has a pointer for each field, and a nil pointer leaves that field unchanged:

```go
type config struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Verbose bool   `json:"verbose"`
	secret  string
}

// wanted, without writing it by hand:
type configPatch struct {
	Name    *string `json:"name,omitempty"`
	Port    *int    `json:"port,omitempty"`
	Verbose *bool   `json:"verbose,omitempty"`
}

func (p configPatch) Apply(c config) config { … }
```

Writing `configPatch` by hand repeats every field of `config`, and it drifts
when `config` changes. The derived type should follow `config`, keep and change
the struct tags, and carry hand-written methods such as `Apply`.

## Solution

> **Strict comptime, not opportunistic.** The source does not build without
> `go-comptime`; the generated file does. This follows
> [the strict proposal](../proposal-alternative-strict-comptime.md), which is
> not approved.

A type function builds the struct from the fields of another. It reads them
the same way `reflect` and the `comptime` package do: `Field` is a struct with
`Name`, `Type` and `Tag`. It takes no arguments, only a type parameter:

```go
// patch describes a struct with a pointer to each field of T; nil leaves the field unchanged.
func patch[T any]() comptime.Type {
	var fields []comptime.Field
	for f := range comptime.TypeFor[T]().Fields() {
		f.Type = comptime.PointerTo(f.Type)
		if name, ok := f.Tag.Lookup("json"); ok && !strings.Contains(name, ",omitempty") {
			f.Tag = reflect.StructTag(`json:"` + name + `,omitempty"`)
		}
		fields = append(fields, f)
	}
	return comptime.StructOf(fields...)
}
```

`Fields` yields only exported fields, so `secret` has no pointer in the patch.
A placeholder and a `Reify` call name the type (see [geo.md](geo.md)), and
methods are written on it by hand:

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

The generated file declares `type configPatch struct { … }` with the computed
fields and tags. `Apply` has one
`if p.Name != nil { c.Name = *p.Name }` for each field, and no reflection.

- Tags are part of `Field`, so they are copied, changed or computed like names
  and types. `Reify` and `Same` compare them, and `go vet` checks them in the
  generated file.
- A reified type is a defined type, so it can have methods. They are written
  by hand and specialized like any function.
- Computed method sets, such as one method for each field, are out of scope.
  They add declarations, and adding declarations is qudy's job.

In the earlier tests, a type `configPatchT` was a hand-written stand-in for
the computed type. It shows what `go-comptime` must generate, and no stub file
of that kind is part of the solution.

## Evidence

- With the `comptime` package running on `reflect`, `patch[config]()`
  describes the struct with pointer fields and the original tags.
  `patch[int]()` panics with `reflect: NumField of non-struct type int`.
- The `omitempty` version matches the stand-in `configPatchT`, tags included.
  `configPatch{Port: &port}.Apply(config{Name: "server", Port: 8080})` returns
  the config with port 9090.
- No generated file for this example was written.

## Open points

- gopls reports uses of computed members, such as `p.Port` on the placeholder,
  as errors. The rest of the file type-checks.
- Whether `go-comptime -w` should write the computed fields back into the
  placeholder declaration.
