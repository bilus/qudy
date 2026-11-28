# Checking struct fields against `validate` tags (opportunistic)

## Problem

A form type carries its rules in struct tags, as with
`github.com/go-playground/validator`:

```go
type signup struct {
	Name  string `validate:"required,min=2"`
	Email string `validate:"email"`
	Age   int    `validate:"gte=18"`
}
```

`validator.Struct(s)` walks the fields with reflection and parses each tag on
every call. Its own caches help, but the rules for `signup` are the same on
every call: only the field values change.

`validator.Validate` itself cannot be marked comptime. It holds functions,
caches and a mutex, and none of them can be written as a literal. Its argument
is `any`, and an interface is not a comptime type (C6).

The goals:
- The rules of each tag are parsed once, at build time.
- Simple rules such as `min` and `gte` become plain comparisons.
- Without `go-comptime`, the same code runs and gives the same answers.

## Solution

> **Opportunistic, but beyond the first version.** The solution needs
> specialization, which R1 to R18 do not cover. The first version cannot hold
> the rules in a tagged variable either: `reflect.Type` is an interface, which
> is not a comptime type (C6), and a slice of rules holds a reference, so by R3
> it cannot leave build time.

The user's code works on a type as a value, not on `any`. A helper takes the
value through `comptime.Value`, whose type `go-comptime` knows. Tag parsing is
a pure function of plain data:

```go
// Rule is one rule of a validate tag: plain data, so it can be comptime.
type Rule struct{ Name, Param string }

// parseRules splits a validate tag into its rules. It is pure, so on a
// comptime tag its result is comptime too.
func parseRules(tag string) []Rule { … }

// check runs each field's rules: min and gte directly, the rest through validate.
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
			case r.Name == "gte" && f.Type.Kind() == reflect.Int:
				n, _ := strconv.ParseInt(r.Param, 10, 64)
				if x.Int() < n {
					return fmt.Errorf("%s: gte=%s", f.Name, r.Param)
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

- Without the tool, the `comptime` package runs on `reflect`, and `check` is an
  ordinary reflective function that any caller can use on any struct.
- With the tool, a call such as `check(comptime.ValueOf(&s), validate)` with
  `s` a `signup` is specialized in the caller's package, as in
  [rowscan-opportunistic.md](rowscan-opportunistic.md):
  - the loops over the fields and over the rules of each tag are unrolled;
  - `min=2` becomes `utf8.RuneCountInString(s.Name) < 2`, and `gte=18` becomes
    `int64(s.Age) < 18`;
  - `required` and `email` still go through the validator, with `any(s.Name)`
    and `any(s.Email)`: `.Reflect().Interface()` is the gate where build time
    ends on purpose.
- A call whose type is not known at build time keeps using plain `check`.

The same pattern fits bennu's forms: instead of a directive with a body,
extract a helper that takes the form's type as a value, and let the tool
specialize it for each form type.

## Evidence

- With the `comptime` package running on `reflect`, `check` agrees with
  `validator.Struct` on six inputs, pass or fail. One of them is a name of one
  rune and two bytes, `"Ż"`, which `min=2` rejects. That version read fields
  through methods, `f.Name()`, before `Field` became a struct.
- In a copy of bennu, a hand-written `FormSync` body for one form type gives the
  same form state as the reflective `Sync` over four steps: an empty form, a
  changed field, no change, and a stale error.
- No specialized copy of `check` was written.

## Open points

- Which validator rules the tool lowers to plain code, and which go through the
  gate.
- Whether `check` needs an opt-in directive to be specialized.
