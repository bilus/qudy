# A small Lisp-to-Go transpiler using Qudy

A small Clojure-like language that compiles to Go.

## Run

Requires Go 1.25 or later.

```sh
go run . examples/hello.lisp | gofmt > /tmp/hello.go
go run /tmp/hello.go
# Hello JOHN (10)

# Read from stdin:
go run . < examples/features.lisp | gofmt > /tmp/features.go

# Regenerate the compiler:
go generate
```

## Syntax

```clojure
(package main)
(import "fmt" "strings")

(defn square ^int64 [^int64 x] (* x x))

(defn main []
  (let [name "John" age 10]
    (fmt.Printf "Hello %s (%d)\n" (strings.ToUpper name) age)))
```

Return-type hints precede parameter vectors. Omit the hint for functions without a result.

| Form | Meaning |
| --- | --- |
| `(defn name ^type [args] body ...)` | Named function. |
| `(fn ^type [args] body ...)` | Closure. |
| `(let [name value ...] body ...)` | Sequential bindings; returns the last expression. |
| `(if condition then else)` | Returns the selected branch. |
| `(when condition body ...)` | Returns the last expression, or the type's zero value. |
| `(f args ...)` | Function call; Go calls use qualified names. |
| `[1 2 3]` | Homogeneous list. |
| `first`, `rest`, `empty?`, `cons` | List operations. |
| `+ - * / %` | Arithmetic; `+` also joins strings. |
| `= not= < <= > >=` | Comparisons. |
| `and or not` | Boolean operations. |

Types include `int64`, `float64`, `string`, `bool`, `error`, and qualified Go types.
Use `^[int64]` for lists and `^(fn [int64] bool)` for function types.

```clojure
^[int64] []
(fmt.Println ^string (if true "yes" "no"))
```

Examples include recursive `map`, `reduce`, and `filter` in `examples/lists.lisp`.

Or just look at this beauty:

``` clojure
;; Print a tree of the given directory, including the size of each file and directory.
(package main)
(import "fmt" "io" "io/fs" "log" "os" "path/filepath" "strings" "sync/atomic")

;; Buffer children so the directory header can include its final size.
;; io.Writer accepts both os.Stdout and the *strings.Builder created below.
(defn tree ^int64 [^string root ^string indent ^io.Writer output]
      (let [total (new atomic.Int64)
           children (new strings.Builder)
           childIndent (+ indent "  ")
           walkError
           (filepath.Walk
            root
            (fn ^error [^string path ^fs.FileInfo info ^error err]
                (if (not= err nil)
                    err
                    (if (= path root)
                        (if (info.IsDir) nil (fmt.Errorf "not a directory: %s" root))
                        (if (info.IsDir)
                            (let [size (tree path childIndent children)]
                                 (total.Add size)
                                 ;; tree has handled this subtree; do not visit it again.
                                 filepath.SkipDir)
                            (let [mode (info.Mode)]
                                 (if (mode.IsRegular)
                                     (let [size (info.Size)]
                                          (total.Add size)
                                          (fmt.Fprintf children "%s%s (%d B)\n" childIndent (info.Name) size))
                                     (fmt.Fprintf children "%s%s (not counted)\n" childIndent (info.Name)))
                                 nil))))))]
           (when (not= walkError nil) (log.Fatal walkError))
           (let [size (total.Load)
                name (filepath.Base root)
                ^string label (if (= name (string filepath.Separator)) name (+ name "/"))]
                (fmt.Fprintf output "%s%s (%d B)\n%s" indent label size (children.String))
                size)))

(defn main []
  (let [args (rest os.Args)
       ^string root (if (empty? args) "." (first args))]
       (tree (filepath.Clean root) "" os.Stdout)))
```

## Deliberate limits

- Only `package`, `import`, and `defn` are allowed at top level.
- Those forms cannot appear inside other expressions.
- Function parameters and value-returning functions require type hints.
- Conditions must be booleans.
- Missing branches and empty bodies return the result type's zero value.
- Lists are homogeneous; empty lists need a type hint or context.
- `first` on an empty list panics.
- Functions are monomorphic; each element type needs its own `map` implementation.
- No macros, destructuring, variadic Lisp functions, or overloads.
- Values used in bindings, arguments, or returns must have one result.
- Mixed numeric types require explicit conversions.
- Some expressions require hints when their result type cannot be determined.
- Names follow Go identifier syntax; `_lispg` is reserved.
- Avoid shadowing Go's predeclared type names.
- Go checks names, imports, calls, and types.
- Errors can leave partial output; use it only after successful compilation.
