# Checking struct fields against `validate` tags (strict)

## Problem

The problem is the one in
[validator-opportunistic.md](validator-opportunistic.md), with different
goals:

- The package checks only its own form types, and nothing outside it calls the
  checker.
- The binary should not parse tags or walk fields at all.
- A malformed rule, such as `min=two`, should stop the build.
- Any reflection left in the binary should be visible in the source.

## Solution

> **Strict comptime, not opportunistic.** The source does not build without
> `go-comptime`; the generated file does. This follows
> [the strict proposal](../proposal-alternative-strict-comptime.md), which is
> not approved.

The source of `check` and `parseRules` is the same as in the opportunistic
version. Its meaning differs in three ways:

- **No fallback.** `go-comptime` generates the specialized check for each form
  type, and the binary holds only that. A call whose type is not known at
  build time is an error at the source line.
- **Errors instead of slow paths.** A panic during evaluation is an error, so
  a rule that `parseRules` or a `panic` in `check` rejects stops the build.
  For example, `check` could panic on a `min` parameter that is not a number.
- **The `Reflect` gate is listed.** `x.Reflect()` leaves build time on
  purpose. `go-comptime` writes `.Reflect().Interface()` as `any(x)` and lists
  the gates that remain in each function. There is no way back from
  `reflect.Value` to `comptime.Value`.

For `Name string` with the tag `validate:"required,min=2"`, the generated
code calls the validator for `required` with `any(s.Name)` and checks
`min=2` with `utf8.RuneCountInString(s.Name) < 2`. A `*validator.Validate`
is never known at build time, so it stays a run-time argument.

Because strict functions do not exist at run time, a library cannot export
this `check` for its callers' types. That case needs the opportunistic
version.

## Evidence

- The evidence of the opportunistic version applies: with the `comptime`
  package running on `reflect`, `check` agrees with `validator.Struct` on six
  inputs.
- No generated file for this example was written. The current `check`
  ignores a bad `min` parameter (`n, _ := strconv.Atoi(...)`), so the
  build error above needs a `panic` added there.
