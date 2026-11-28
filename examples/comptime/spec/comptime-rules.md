# Comptime rules

A comptime rule says when one kind of expression or statement is comptime.
R18 in [requirements.md](requirements.md) uses the rules: the analyzer derives
that a function is comptime when every expression and statement in its body
is comptime.

This file holds only approved rules. A proposed rule enters after the project
owner approves its wording. It then gets the next number, C1, C2 and so on.

## How to read a rule

Each rule has four parts:

- **Rule:** one sentence that says when a kind of expression or statement is
  comptime, or that it is not.
- **Derived:** a function that the analyzer can derive with the rule.
- **Not derived:** a function that shows where the rule ends.
- **Notes:** optional.

The analyzer on branch `comptime` agrees with every "Derived" and "Not
derived" below. It judges exported functions, so the examples have exported
names.

## Comptime types

A comptime type is a type that a value in comptime code may have. Rule C6
gives the list its effect.

Approved 2026-11-27.

| Type | Comptime type | Why, or with which limit |
| --- | --- | --- |
| `bool` | yes | |
| `int8` to `int64`, `uint8` to `uint64`, with `byte` and `rune` | yes | |
| `int`, `uint`, `uintptr` | yes, with a condition | Host and target must agree on the size of `int`. |
| `string` | yes | |
| array or struct of comptime types | yes | |
| slice of, or pointer to, a comptime type | yes | Its value holds a reference, so by R3 it cannot leave build time. |
| defined type whose underlying type is a comptime type | yes | |
| `float32`, `float64`, `complex64`, `complex128` | no | Floating-point results can differ between host and target. |
| map, channel, function, interface | no, in the first version | |
| `unsafe.Pointer` | no | |

**Notes.**

- The list is stricter than the analyzer on branch `comptime` in one row. The
  analyzer accepts a map, a channel, a function value or an interface when a
  function only passes it on, and rejects it when the function uses it.
- The analyzer gives `int` no condition when the value only passes through. It
  adds the condition when the function computes with the value or converts to
  `int`.

## Rules

### C1. Constant expressions

Approved 2026-11-27.

**Rule.** A constant expression is comptime.

**Derived.**

```go
func Lit() int64 { return 42 }
```

**Notes.**

- "Constant expression" is the Go specification's term, and a literal is one.
- The rule does not yet say what follows for a constant whose value depends
  on the target, such as `math.MaxInt`. The analyzer gives such a fact a
  condition.

### C2. Parameters and local variables

Approved 2026-11-27.

**Rule.** A reference to a parameter or to a local variable of the function is
comptime.

**Derived.**

```go
func Param(width int64) int64 { return width }
```

**Not derived.**

```go
var Global int64 = 3

func ReadsGlobal() int64 { return Global }
```

`Global` is a package-level variable, which this rule does not cover.

### C3. Integer and boolean operations

Approved 2026-11-27.

**Rule.** An arithmetic, shift, comparison or logical operation on integer or
boolean operands is comptime when its operands are.

**Derived.**

```go
func Arith(a, b int64) int64 { return a<<1 - b }
```

**Not derived.**

```go
func Half(x float64) float64 { return x / 2 }
```

The operands are floating-point values.

**Notes.**

- A division can panic. A comptime expression may panic, and R1 says what
  happens when its evaluation does.
- For operands of type `int`, `uint` or `uintptr`, the fact carries the
  condition that the list of comptime types names.

### C4. Conversions between integer types

Approved 2026-11-27.

**Rule.** A conversion from one integer type to another is comptime when its
operand is.

**Derived.**

```go
func Conv(width int64) uint64 { return uint64(width) }
```

**Not derived.**

```go
func ConvStrBytes(x string) []byte { return []byte(x) }
```

No rule covers this conversion, although both types are comptime types. The
Go specification leaves the capacity of the new slice open.

### C5. Return statements

Approved 2026-11-27.

**Rule.** A `return` statement is comptime when its results are.

**Derived.**

```go
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }
```

The derivation uses C1 to C5: `1` is a constant expression, `width` is a
parameter, `uint64(width)` converts between integer types, the shift and the
subtraction are integer operations, and the `return` statement holds them.

### C6. The type of an expression

Approved 2026-11-27.

**Rule.** An expression is comptime only if its type is a comptime type.

**Not derived.**

```go
func Float64(x float64) float64 { return x }
```

The function computes nothing, and `x` is a parameter. Its type is not a
comptime type.

**Notes.**

- Every other rule says "when". This one says "only if", so it can only take
  a derivation away.

### C7. References to functions

Approved 2026-11-27.

**Rule.** A reference to a function or method is comptime when that function
or method is comptime.

**Derived.**

```go
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

func Twice(width int64) uint64 { return Mask(width) + Mask(width) }
```

**Not derived.**

```go
func Env() int64 { return int64(len(os.Getenv("WIDTH"))) }
```

`os.Getenv` is not comptime.

**Notes.**

- For the analyzer, a function is comptime when it has a fact: a trusted one
  from a directive (R14), or an inferred one.
- This is the rule behind "os.Getenv is not comptime" in the warning of R15.
- A function may refer to itself. The analyzer derives a function that calls
  itself.

### C8. Static calls

Approved 2026-11-27.

**Rule.** A static call is comptime when the reference to the function and the
arguments are.

**Derived.**

```go
type Meter int64

func (m Meter) Double() Meter { return m * 2 }

func CallsMethod(m Meter) Meter { return m.Double() }
```

**Not derived.**

```go
func Dynamic(f func() int64) int64 { return f() }
```

The call goes through a function value, so it is a dynamic call.

**Notes.**

- C6 already excludes the parameter `f`, because a function type is not a
  comptime type.

### C9. Package-level variables

Approved 2026-11-27.

**Rule.** A reference to a package-level variable is not comptime.

**Not derived.**

```go
var Global int64 = 3

func ReadsGlobal() int64 { return Global }
```

**Notes.**

- The rule holds for the first version. A variable that is not exported and
  is assigned only by its declaration can count as comptime; that needs a rule
  of its own.
