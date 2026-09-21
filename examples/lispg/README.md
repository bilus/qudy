# A small Lisp-to-Go transpiler using Qudy

The compiler parses a (GREATLY) simplified version of Clojure and transpiles it
to Go.

## Run

```sh
go build -o lispg .
go run . examples/hello.lisp > /tmp/hello.go
gofmt -w /tmp/hello.go
go run /tmp/hello.go
# Hello JOHN (10)

# stdin works too:
cat examples/features.lisp | go run . > /tmp/features.go
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

Only `package`, `import`, and `defn` can appear at top level, and those forms
cannot appear inside other expressions. The compiler handles them in
`namespaceExpr`. All local forms are expressions. `discardExpr` evaluates an
expression when its result is unused; `let`, `if`, and `when` retain the same
meaning in either context. An empty body yields the result type's zero value.

Function arguments require types. Function results default to no result. Use
`int64`, `float64`, `string`, or `bool`; booleans provide conditions. Integer
literals emit `int64(...)`, floating literals emit `float64(...)`. Strings use
Go-compatible double-quoted escapes. Semicolons start comments; commas are
whitespace. Names must be Go identifiers; qualified calls use dots.

```clojure
(defn ^int64 factorial [^int64 n]
  (if (<= n 1) 1 (* n (factorial (- n 1)))))

(let [x 10 add (fn ^int64 [^int64 y] (+ x y))]
  (fmt.Println (add 5)))
```

`if`, `when`, and `let` emit immediately invoked Go functions when their values
are needed. When a value is discarded, the compiler emits the body directly.
Their result type comes from the enclosing typed return, or a `^type` hint. An
ordinary call does not tell the transpiler its argument types, so give a hint
when passing one of these forms as an argument:

```clojure
(fmt.Println ^string (if true "yes" "no"))
(let [^int64 x (when true 10)] (fmt.Println x))
(fmt.Println ^int64 (when false 42)) ; 0
(fmt.Println ^int64 (let [x 40] (+ x 2))) ; 42
```

An explicit type on a binding also declares that Go variable's type. Hints on
expressions guide conditional emission; they do not perform numeric conversions.
Use ordinary conversion calls such as `(float64 x)` when needed.

## Deliberate limits

This is a Clojure-flavored subset with Go typing. Conditions must be booleans.
`when` uses a typed zero (`0`, `0.0`, `""`, or `false`) for its missing branch.
There is no Clojure `nil`/truthiness model, collection runtime, macros,
destructuring, variadic Lisp functions, or function overloads. Parameters and
results use scalar types; a local `let` can infer a closure's Go function type.
Calls to Go functions with zero or multiple results may be evaluated with their
results discarded; a value used by a binding, argument, or return must be a
single value. Mixed numeric types need explicit conversions.

Go resolves names and checks call signatures and types. Avoid shadowing Go's
predeclared type names, since literal emission uses `int64` and `float64`. The
reader reports structural errors with source positions. Since emission is
direct, a later error can leave partial output on stdout; only use that output
when the transpiler exits successfully. Run `gofmt` separately for indentation.
