# A small Lisp-to-Go transpiler using Qudy

The compiler parses a (GREATLY) simplified version of Clojure and transpiles it
to Go.

## Run

```sh
go run . examples/hello.lisp | gofmt > /tmp/hello.go
go run /tmp/hello.go
# Hello JOHN (10)

# stdin works too:
cat examples/features.lisp | go run . | gofmt > /tmp/features.go
go run /tmp/features.go
```

To regenerate the compiler code:

```sh
go generate
```

## Syntax

```clojure
(package main)
(import "fmt" "strings")

(defn main []
  (let [a 10 b "John"]
    (fmt.Printf "Hello %s (%d)\n" (strings.ToUpper b) a)))
```

This adjusts the original example so it runs: `main` is the executable package,
both imports are used, and `%s` receives a string. `(package foo)` is also valid
when producing a library. Imports are preserved; Go reports unused imports.

| Form                                         | Meaning                                                                              |
|----------------------------------------------|--------------------------------------------------------------------------------------|
| `(defn name [^type arg ...] body ...)`       | Declare a function with no result.                                                   |
| `(defn ^type name [^type arg ...] body ...)` | Return the last body expression.                                                     |
| `(fn ^type [^type arg ...] body ...)`        | Anonymous function, with lexical capture. Omit the vector's type hint for no result. |
| `(let [name value ...] body ...)`            | Sequential bindings in nested lexical scopes; returns the last body expression.      |
| `(if condition then else)`                   | Returns the selected branch's value.                                                 |
| `(if condition then)`                        | Returns the branch value when true, otherwise the result type's zero value.          |
| `(when condition body ...)`                  | Returns the last body expression when true, otherwise the result type's zero value.  |
| `(f arg ...)`, `(fmt.Println arg ...)`       | Local or qualified Go call.                                                          |
| `+ - * / %`                                  | Arithmetic; `+` also concatenates strings. Unary `-` is supported.                   |
| `= not= < <= > >=`                           | Binary comparisons.                                                                  |
| `and or not`                                 | Boolean operations; `and` and `or` short-circuit.                                    |

Return-type hints go immediately before the parameter vector for both `defn`
and `fn`. A hint on a `defn` name is rejected. Function arguments require
types. Function results default to no result. Use
`int64`, `float64`, `string`, or `bool`; booleans provide conditions. Go interop
also accepts `error` and qualified type names such as `fs.FileInfo` and
`io.Writer`. The Go compiler checks those names against the imports. Integer
literals emit `int64(...)`, floating literals emit `float64(...)`. Strings use
Go-compatible double-quoted escapes. Semicolons start comments; commas are
whitespace. Names use Go identifier syntax; qualified calls use dots. Go keywords
such as `map` are renamed in generated code. The `_lispg` prefix is reserved.

```clojure
(defn factorial ^int64 [^int64 n]
  (if (<= n 1) 1 (* n (factorial (- n 1)))))

(let [x 10 add (fn ^int64 [^int64 y] (+ x y))]
  (fmt.Println (add 5)))
```

`if`, `when`, and `let` emit immediately invoked Go functions when their values
are needed. When a value is discarded, the compiler emits the body directly.
Their result type comes from the enclosing typed return, or a `^type` hint.
An ordinary call does not tell the transpiler its argument types, so give a
hint when passing one of these forms as an argument:

```clojure
(fmt.Println ^string (if true "yes" "no"))
(let [^int64 x (when true 10)] (fmt.Println x))
(fmt.Println ^int64 (when false 42)) ; 0
(fmt.Println ^int64 (let [x 40] (+ x 2))) ; 42
```

An explicit type on a binding also declares that Go variable's type. Hints on
expressions guide conditional emission; they do not perform numeric conversions.
Use ordinary conversion calls such as `(float64 x)` when needed.

## Homogeneous lists and function types

List literals use brackets. Each list has one element type; nested lists work too.
The generated Go compiler infers a nonempty list's type from its elements:

```clojure
[1 2 3]          ; []int64
[1.5 2.5]        ; []float64
["hello" "world"] ; []string
[true false]     ; []bool
[[1 2] [3 4]]    ; [][]int64
```

Mixed lists such as `[1 "hello"]` or `[1 2.5]` fail Go's type checks. Integer
and floating literals keep their distinct types; convert explicitly if needed.
For an empty list, supply a hint or a known result/binding type:

```clojure
^[int64] []
(let [^[string] names []] (empty? names))
(defn numbers ^[int64] [] [])
```

Hints describe types with the same reader forms as ordinary code:

| Hint | Go type |
| --- | --- |
| `^[int64]` | `[]int64` |
| `^[[string]]` | `[][]string` |
| `^(fn [int64] bool)` | `func(int64) bool` |
| `^(fn [int64 int64] int64)` | `func(int64, int64) int64` |

Function type hints require one result type. Parameters and results can
also be lists or functions. For example, `map` takes a typed function:

```clojure
(defn map ^[int64] [^(fn [int64] int64) f ^[int64] xs]
  (if (empty? xs)
    []
    (cons (f (first xs)) (map f (rest xs)))))
```

`examples/lists.lisp` implements recursive `map`, `reduce`, and `filter`,
including a closure that captures a local variable. These are ordinary
functions written in Lisp; they are not compiler built-ins.

```sh
./lispg examples/lists.lisp > /tmp/lists.go
go run /tmp/lists.go
# map: [1 4 9 16]
# reduce: 10
# filter: [2 4]
```

| Primitive | Behavior |
| --- | --- |
| `(first xs)` | Returns the first element; an empty list causes a Go bounds panic. |
| `(rest xs)` | Returns the tail; an empty list stays empty. |
| `(empty? xs)` | Returns whether the list has no elements. |
| `(cons x xs)` | Returns a fresh list containing `x` followed by `xs`. |

`cons` copies the outer slice and leaves its input unchanged. `rest` shares
backing storage; the language supplies no list mutation operation. Nested
lists still share their inner slices. Go interop that mutates a slice can
therefore affect aliases.

Five small generic helpers are emitted into each Go program. Their `T any`
constraint allows different element types across calls; every individual
list remains a `[]T`, never a `[]any`. Go checks homogeneity and function
signatures without runtime type assertions. Empty lists cannot infer a type
from a neighboring call argument; add `^[type]` when no result or binding
context supplies the type.

## Deliberate limits

This is a Clojure-flavored subset with Go typing. Conditions must be booleans.
`when` uses the result type's zero value for its missing branch: numeric zero,
`""`, `false`, or Go `nil` for slices and functions. A nil slice behaves as an
empty list. There is no Clojure truthiness model, macros, destructuring,
variadic Lisp functions, or function overloads. Lisp functions are monomorphic;
the example `map` handles `int64` lists. Other element types need typed versions. Calls to Go functions with zero or multiple results may be evaluated
with their results discarded; a value used by a binding, argument, or return
must be a single value. Mixed numeric types need explicit conversions.

Go resolves names and checks call signatures and types. Avoid shadowing Go's
predeclared type names, since literal emission uses `int64` and `float64`.
The reader reports structural errors with source positions. Since emission is
direct, a later error can leave partial output on stdout; only use that output
when the transpiler exits successfully. Run `gofmt` separately for indentation.
