# Requirements

[README.md](README.md) explains the form of a requirement, and
[vocabulary.md](vocabulary.md) defines the terms.

The requirements stand in two sections, one for each layer:

- [Binding-time analysis](#binding-time-analysis) decides whether an
  expression is comptime. It holds R1 to R3 and R14 to R18.
- [The driver, `go-comptime`](#the-driver-go-comptime) writes files and runs
  the `go` command. It holds R4 to R13.

## Binding-time analysis

This section says when an expression is comptime and when tagged code is an
error. Partial evaluation is the technique of running part of a program ahead
of time. Its literature calls this decision binding-time analysis, because the
decision gives each expression a binding time: comptime or run-time.

### R1. A panic during requested evaluation is an error

Approved 2026-11-27.

**Requirement.** If the initialization expression of a tagged variable panics
when `go-comptime` evaluates it, then `go-comptime` shall report an error that
gives the position of the expression and the panic value.

**Example.**

```go
package main

//qudy:comptime
func Pick(xs []string, i int) string { return xs[i] }

//qudy:comptime
var second = Pick([]string{"a", "b"}, 5)

func main() { println("main runs:", second) }
```

**Output and behavior.**

```
p.go:7:14: comptime: evaluation panics: runtime error: index out of range [5] with length 2
```

The error gives the position of the initialization expression, `Pick(...)` on
line 7, and not the position of `xs[i]` inside `Pick`, where the panic starts.
R5 says what follows the error.

**Notes.**

- Without `go-comptime`, this program compiles. It then panics with the same
  message during package initialization, before `main` runs.
- Go and Zig do the same when they must evaluate an expression at compile
  time. Go rejects `a[5]` for an array of two elements, and Zig rejects
  `comptime pick("ab", 5)`.
- R14 covers the tagged function in the example.

### R2. A directive on a package-level variable requests evaluation

Approved 2026-11-27.

**Requirement.** When `go-comptime` processes a package, `go-comptime` shall
evaluate the initialization expression of each tagged package-level variable.

**Example.**

```go
package p

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

var wordMask = Mask(16)
```

**Output and behavior.**

`go-comptime` evaluates `Mask(8)` to the `uint64` value 255. It does not
evaluate `Mask(16)`, because `wordMask` is not tagged. R6 says what
`go-comptime` writes.

**Notes.**

- The directive has the form of `//go:generate`: no space follows `//`, and
  `gofmt` leaves such a line as it is.
- The variable stays a variable. `go-comptime` evaluates only its
  initialization expression.

### R3. A result that holds a reference cannot leave build time

Approved 2026-11-27.

**Requirement.** If the initialization expression of a tagged variable has a
type that is not self-contained, then `go-comptime` shall report an error
that gives the position of the expression and names the type.

**Example.**

```go
package p

//qudy:comptime
func Primes() []int64 { return []int64{2, 3, 5, 7} }

//qudy:comptime
var primes = Primes()

//qudy:comptime
var third = Primes()[2]
```

**Output and behavior.**

```
p.go:7:14: comptime: the result has type []int64, which holds a reference and cannot leave build time
```

R5 says what follows the error. Without the declaration of `primes`,
`go-comptime` accepts the file and evaluates `Primes()[2]` to the `int64`
value 5. The slice exists only while `go-comptime` evaluates the expression,
and the result is self-contained.

**Notes.**

- A self-contained type is a boolean, numeric or string type, or an array or
  struct of self-contained types.
- Run-time code can still call `Primes` like any function.
- Zig has this rule for a pointer to a `comptime var`. A later version can
  relax it: `go-comptime` would write a composite literal, which creates a new
  variable each time the program evaluates it.

### R14. A tagged function is trusted

Approved 2026-11-27.

**Requirement.** `go-comptime` shall hold a trusted fact for each tagged
function.

**Example.**

```go
package main

import (
	"fmt"
	"os"
)

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
func Width() int64 { return int64(len(os.Getenv("WIDTH"))) }

//qudy:comptime
var mask = Mask(Width())

func main() { fmt.Println(mask) }
```

**Output and behavior.**

```
$ WIDTH=abc go-comptime build .
p.go:12:39: comptime: warning: os.Getenv is not comptime
	p.go:11:1: Width is tagged here
	p.go:15:17: evaluated at build time from here
$ WIDTH=abcdefgh ./p
7
$ WIDTH=abcdefgh go run .
255
```

`go-comptime` evaluates `Mask(Width())` at build time, although the analyzer
finds no derivation for `Width`. The comptime file therefore holds the value
that `WIDTH` had during the build.

**Notes.**

- The example shows what trust costs. The fact about `Width` is false, so the
  program that `go-comptime` built and the program that `go` built behave
  differently. Both builds behave the same only while every trusted fact is
  true.
- The warning is the one of R15.
- To run a tagged function, the generator needs every function that it
  reaches, tagged or not.
- The two programs were run here without `go-comptime`: `WIDTH=abc go run .`
  prints 7, and `WIDTH=abcdefgh go run .` prints 255 (Go 1.27.4).

### R15. A trusted fact without a derivation is a warning

Approved 2026-11-27.

**Requirement.** If the analyzer finds no derivation for the trusted fact of a
tagged function, then `go-comptime` shall report a warning that names the
function and gives the position of the reference or construct that prevents
the derivation.

**Example.**

```go
package main

import (
	"fmt"
	"os"
)

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
func Width() int64 { return int64(len(os.Getenv("WIDTH"))) }

//qudy:comptime
var mask = Mask(Width())

func main() { fmt.Println(mask) }
```

**Output and behavior.**

```
p.go:12:39: comptime: warning: os.Getenv is not comptime
	p.go:11:1: Width is tagged here
	p.go:15:17: evaluated at build time from here
```

The first line names what prevents the derivation and gives its position. The
indented lines give the directive that asserted the fact and the place from
which `go-comptime` evaluates the function. The third line is absent when no
tagged variable uses the function.

`go-comptime` reports nothing for `Mask`, because the analyzer derives that
`Mask` is comptime. After the warning, `go-comptime` goes on as if the
analyzer had found a derivation.

**Notes.**

- The warning has the shape of Zig's report in Go's form. Zig puts its error
  at the operation that cannot run at compile time and adds a note for each
  step outwards. Go's compiler prints a second position as an indented line.
- R18 says which derivations the analyzer can find.
- The analyzer on branch `comptime` finds this reason today: for `Width` it
  reports the call of `os.Getenv`, and nothing for `Mask`.
- A function that is comptime but lies beyond the analyzer gets this warning
  too. No requirement offers a way to silence it.

### R16. `-strict` turns the warnings of the analysis into errors

Approved 2026-11-27.

**Requirement.** Where `go-comptime` is invoked with `-strict`, `go-comptime`
shall report each warning of the binding-time analysis as an error.

**Example.**

```go
package main

import (
	"fmt"
	"os"
)

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
func Width() int64 { return int64(len(os.Getenv("WIDTH"))) }

//qudy:comptime
var mask = Mask(Width())

func main() { fmt.Println(mask) }
```

**Output and behavior.**

```
$ go-comptime -strict build .
p.go:12:39: comptime: os.Getenv is not comptime
	p.go:11:1: Width is tagged here
	p.go:15:17: evaluated at build time from here
exit status 1
```

By R5, `go-comptime` then writes no file.

**Notes.**

- With `-strict`, a directive on a function is a claim that the analyzer
  checks, as `const fn` is in Rust. Without it, the directive is a promise
  that `go-comptime` trusts.
- The flag stands before the subcommand. R7 passes the arguments after the
  subcommand to the `go` command.
- The flag reaches the warnings of R15 and R17. A warning of the driver stays
  a warning.

### R17. A tagged variable follows the same rule

Approved 2026-11-27.

**Requirement.** If the analyzer cannot derive that the initialization
expression of a tagged variable is comptime, then `go-comptime` shall report a
warning that names the variable and gives the position of the reference or
construct that prevents the derivation.

**Example.**

```go
package main

import (
	"fmt"
	"os"
)

//qudy:comptime
var home = os.Getenv("HOME")

func main() { fmt.Println(home) }
```

**Output and behavior.**

```
p.go:9:12: comptime: warning: os.Getenv is not comptime
	p.go:8:1: home is tagged here
```

`go-comptime` still evaluates the expression, as R2 says, and the comptime
file holds the value that `HOME` had during the build. With `-strict`, the
warning is an error, by R16.

**Notes.**

- This is R15 for a variable. The directive on `home` asserts that its
  initialization expression is comptime, and the analyzer looks for a
  derivation.

### R18. What the analyzer can derive

Approved 2026-11-27.

**Requirement.** The analyzer shall derive that a function is comptime when
every expression and statement in its body is comptime.

**Example.**

```go
package main

import (
	"fmt"
	"os"
)

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
func Width() int64 { return int64(len(os.Getenv("WIDTH"))) }

//qudy:comptime
var mask = Mask(Width())

func main() { fmt.Println(mask) }
```

**Output and behavior.**

The analyzer derives that `Mask` is comptime: its body is one `return`
statement, and every expression in it is comptime. It derives nothing for
`Width`, because the call `os.Getenv("WIDTH")` is not comptime. So
`go-comptime` reports the warning of R15 for `Width` and none for `Mask`.

**Notes.**

- The comptime rules say when one kind of expression or statement is
  comptime. They stand in [comptime-rules.md](comptime-rules.md).
- The initialization expression of a tagged variable is an expression, so the
  comptime rules decide about it too. R17 depends on that.
- A reference to a function is an expression. It is comptime when a fact says
  that the function is comptime.
- The analyzer on branch `comptime` derives `Mask` today and finds the call of
  `os.Getenv` in `Width`.

## The driver, `go-comptime`

This section says what `go-comptime` does with the result of the analysis:
which files it writes, how it runs the `go` command, and which positions Go
then reports.

### R4. Go reports positions in the source file

Approved 2026-11-27.

**Requirement.** `go-comptime` shall write each comptime file so that the
compiler, `go vet` and run-time tracebacks report every position outside the
literals that replace tagged expressions with the file name, line and column
that the position has in the source file.

**Example.**

```go
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() {
	fmt.Printf("%d\n", "mask")
	var bits []int
	fmt.Println(bits[byteMask])
}
```

The comptime file, `.comptime/p.go`:

```go
// Code generated by go-comptime from p.go. DO NOT EDIT.

//line ../p.go:1:1
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = /*line ../p.go:9:16*/uint64(0xff)/*line ../p.go:9:23*/

func main() {
	fmt.Printf("%d\n", "mask")
	var bits []int
	fmt.Println(bits[byteMask])
}
```

**Output and behavior.**

```
$ go-comptime vet .
p.go:12:14: fmt.Printf format %d has arg "mask" of wrong type string
$ go-comptime run .
%!d(string=mask)
panic: runtime error: index out of range [255] with length 0

goroutine 1 [running]:
main.main()
	/tmp/r4/p.go:14 +0x5f
exit status 2
```

`go vet .` and `go run .` print the same. Both positions are those of `p.go`,
although the comptime file is three lines longer: there the two statements
stand on lines 15 and 17.

**Notes.**

- Go's line directives do the work. The `//line` comment after the header
  gives the next line its position in `p.go`. Each `/*line*/` comment gives a
  position to the text right after it, so the literal starts where the tagged
  expression started, and the rest of the line keeps its column.
- The requirement leaves out positions inside a literal, because a literal
  has no counterpart in the source file.
- The file name in a directive is the path from the comptime file to the
  source file. A bare `p.go` would name a file in `.comptime`. An absolute
  path works, but it differs from machine to machine.
- A directive needs the column. Without one, the compiler drops the column
  from every position after the directive.
- `gofmt` must not format a comptime file. It inserts a space after a
  `/*line*/` comment and can break a line, and both move text away from the
  position that the directive states.
- The output above comes from a comptime file written by hand and built with
  `go vet -overlay` and `go run -overlay` on Go 1.27.4. For a declaration
  added on line 17 of `p.go`, the compiler reported `./p.go:17:14`. With the
  header and no directives, `go vet` named the comptime file at line 14, and
  the traceback gave `p.go:16`.
- R7 says that `go-comptime` builds the package through an overlay.

### R5. An error stops the driver

Approved 2026-11-27.

**Requirement.** If the binding-time analysis reports an error, then
`go-comptime` shall write no file and shall exit with a non-zero status.

**Example.**

```go
package p

//qudy:comptime
func Primes() []int64 { return []int64{2, 3, 5, 7} }

//qudy:comptime
var primes = Primes()

//qudy:comptime
var third = Primes()[2]
```

**Output and behavior.**

```
$ go-comptime build .
p.go:7:14: comptime: the result has type []int64, which holds a reference and cannot leave build time
exit status 1
```

`go-comptime` writes no comptime file for `p.go`, although `Primes()[2]` on
line 10 evaluates to 5. One error is enough to stop every file.

**Notes.**

- The error is the one of R3. An error of R1 stops `go-comptime` in the same
  way.
- R1 and R3 held this behavior until it moved here, so that each of them
  states a verdict and nothing else.
- The requirement does not say what happens to a comptime file that an earlier
  run wrote, or whether `go-comptime` still runs the `go` command.
- R7 says that `go-comptime` accepts the arguments of the `go` command.

### R6. `go-comptime` writes a comptime file for each source file with a tagged variable

Approved 2026-11-27.

**Requirement.** `go-comptime` shall write, for each source file that has a
tagged variable, a comptime file under `.comptime`: a copy of the source file
in which a literal of its value replaces the initialization expression of each
tagged variable.

**Example.**

```go
package p

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

var wordMask = Mask(16)
```

**Output and behavior.**

`go-comptime` writes `.comptime/p.go`:

```go
// Code generated by go-comptime from p.go. DO NOT EDIT.

//line ../p.go:1:1
package p

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = /*line ../p.go:7:16*/uint64(0xff)/*line ../p.go:7:23*/

var wordMask = Mask(16)
```

Apart from comments, the two files differ in one line:

```
< var byteMask = Mask(8)
> var byteMask = uint64(0xff)
```

`wordMask` keeps its expression, because it is not tagged. `Mask` stays in the
file, so that run-time code can still call it.

**Notes.**

- The literal carries its type. A bare `0xff` would make `byteMask` an `int`.
- The header and the line directives are comments. R4 is the reason for the
  directives; no requirement calls for the header.
- A source file without a tagged variable gets no comptime file.
- R5 says when `go-comptime` writes no file at all.
- The requirement does not say which form the literal takes for each
  self-contained type, or where `.comptime` stands when a module has several
  packages.
- The comptime file above was written by hand. With an overlay, `go vet` and
  a test accepted it and saw the `uint64` values 255 and 65535, as they did
  for `p.go` (Go 1.27.4).

### R7. `go-comptime` wraps the `go` command and adds an overlay

Approved 2026-11-27.

**Requirement.** When `go-comptime` is invoked with the `go` subcommand
`build`, `install`, `run`, `test` or `vet`, `go-comptime` shall run that
subcommand with the same arguments and with an overlay that maps each source
file to its comptime file.

**Example.**

```go
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() { fmt.Println(byteMask) }
```

**Output and behavior.**

```
$ go run .
255
$ go-comptime run .
255
```

Both commands print the same. The second writes `.comptime/p.go`, as R6 says,
and then runs `go run -overlay` with an overlay that holds one entry:

```json
{"Replace": {"/tmp/r7/p.go": "/tmp/r7/.comptime/p.go"}}
```

The `go` command therefore compiles the comptime file in place of `p.go`.

**Notes.**

- The overlay names both files by absolute path. Go resolves a relative path
  against the working directory, and from a subdirectory it ignored such an
  overlay without a message. So the overlay differs from machine to machine.
- R10 says where `go-comptime` keeps the overlay. No requirement says what
  `go-comptime` does when the arguments already hold an `-overlay` flag.
- The requirement names its subcommands, because more subcommands accept
  build flags than compile a package. `go fix` is one of them. With the
  overlay it proposed no fix for `p.go`: it skips the comptime file because of
  the "Code generated" header. So `go fix` falls under R8.
- [limitations.md](limitations.md) describes what `-cover` does to the
  overlay.
- The output above comes from a comptime file and an overlay written by hand,
  with Go 1.27.4. `go build -x` named `.comptime/p.go` as the file it compiled.

### R8. Other subcommands pass to the `go` command unchanged

Approved 2026-11-27.

**Requirement.** When `go-comptime` is invoked with any other `go`
subcommand, `go-comptime` shall run that subcommand with the same arguments.

**Example.**

```
$ go-comptime version
```

**Output and behavior.**

```
go version go1.27.4 linux/amd64
```

`go-comptime` runs `go version` and adds nothing. With an overlay the command
would fail, because `go version` rejects the flag:

```
$ go version -overlay overlay.json
flag provided but not defined: -overlay
```

**Notes.**

- "Other" means every subcommand that R7 does not name. `go fix`, `go fmt`
  and `go mod` are among them, so they read and change the source files.
- R12 says what `go-comptime` does after such a subcommand has changed a
  source file.

### R9. `go-comptime` writes only under `.comptime`

Approved 2026-11-27.

**Requirement.** Apart from what the `go` subcommand writes, `go-comptime`
shall change no file outside the `.comptime` directory.

**Example.**

```go
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() { fmt.Println(byteMask) }
```

**Output and behavior.**

```
$ go-comptime build .
$ ls -A
.comptime  go.mod  p  p.go
$ go list ./...
example.com/p
```

`p.go` and `go.mod` hold the same bytes before and after the command.
`go-comptime` wrote the files in `.comptime`. The `go` subcommand wrote the
binary `p`, as `go build .` does.

**Notes.**

- The exception covers every file that the `go` subcommand writes: a binary,
  the build cache, the source files under `go-comptime fmt`, and `go.mod`
  under `go-comptime mod tidy`.
- The `go` command skips `.comptime` in `./...`, because the name starts with
  a dot. So the comptime files never form a package of their own.
- The output above comes from `go build -overlay` with a comptime file written
  by hand, on Go 1.27.4.

### R10. The overlay is kept in `.comptime/overlay.json`

Approved 2026-11-27.

**Requirement.** `go-comptime` shall keep the overlay in
`.comptime/overlay.json`.

**Example.**

```go
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() { fmt.Println(byteMask) }
```

**Output and behavior.**

```
$ go-comptime run .
255
$ cat .comptime/overlay.json
{"Replace": {"/tmp/r7/p.go": "/tmp/r7/.comptime/p.go"}}
```

**Notes.**

- With this location, the overlay falls under R9 like every other file that
  `go-comptime` writes.
- The overlay holds absolute paths, as the notes of R7 explain. A copy from
  another machine or another directory is therefore wrong.
- The requirement does not say when `go-comptime` writes the overlay.

### R11. The template and the generator stay beside the comptime file

Approved 2026-11-27.

**Requirement.** When `go-comptime` writes a comptime file, `go-comptime`
shall also write, in the same directory, the qudy template and the generator
that produced it.

**Example.**

```go
package p

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

var wordMask = Mask(16)
```

**Output and behavior.**

```
.comptime/
    p_qudy.go   the template: the code that runs at build time, and the output around it
    p_gen.go    the generator, which qudy compiled from the template
    p.go        the comptime file, which the generator wrote
```

The template, `.comptime/p_qudy.go`:

```go
//go:build qudy

// Template written by go-comptime from p.go.
package main

import (
	"fmt"
	"os"
)

// Copied from p.go: a tagged function runs in the generator.
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

// eval evaluates a tagged expression and reports a panic as an error at its position.
func eval[T any](pos string, f func() T) (v T) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "%s: comptime: evaluation panics: %v\n", pos, r)
			os.Exit(1)
		}
	}()
	return f()
}

func main() {
	//`// Code generated by go-comptime from p.go. DO NOT EDIT.
	//`
	//`//line ../p.go:1:1
	//`package p
	//`
	//`//qudy:comptime
	//`func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }
	//`
	//`//qudy:comptime
	//`var byteMask = /*line ../p.go:7:16*/uint64(~{fmt.Sprintf("%#x", eval("p.go:7:16", func() uint64 { return Mask(8) }))})/*line ../p.go:7:23*/
	//`
	//`var wordMask = Mask(16)
}
```

Anyone can run the generator again and compare its output with the comptime
file. The command prints nothing:

```
$ go run .comptime/p_gen.go | diff - .comptime/p.go
```

**Notes.**

- These files are the audit trail. The template shows which expression ran at
  build time, because that expression stands inside `~{...}`. To check the
  result, an auditor needs only the `go` command.
- The template copies each tagged function and every function that it
  reaches, because the generator must run them.
- `eval` turns a panic into the error of R1.
- Partial evaluation calls such a generator a generating extension: a program
  that takes the build-time inputs and writes the specialized program.
- The requirement fixes neither the file names nor the content of the
  template. A source file named `p_gen.go` or `p_qudy.go` would collide with
  the names above.
- The template above was written by hand. The `qudy` command compiled it, and
  the generator wrote the comptime file that R6 shows (Go 1.27.4).

### R12. A changed source file gets a new comptime file

Approved 2026-11-27.

**Requirement.** When a `go` subcommand that `go-comptime` ran has changed a
source file, `go-comptime` shall write the comptime file of that source file
again.

**Example.**

```go
package main

import "fmt"



//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() {
	var bits []int
	fmt.Println(bits[byteMask])
}
```

**Output and behavior.**

```
$ go-comptime fmt .
p.go
```

`go fmt` removes two of the three blank lines after the import, so every later
line of `p.go` moves up by two. `go-comptime` then writes `.comptime/p.go`
again. Apart from the two blank lines, the new comptime file differs in its
line directives:

```
< var byteMask = /*line ../p.go:11:16*/uint64(0xff)/*line ../p.go:11:23*/
> var byteMask = /*line ../p.go:9:16*/uint64(0xff)/*line ../p.go:9:23*/
```

**Notes.**

- The requirement names no subcommand, because the set cannot be listed.
  `go fmt` and `go fix` change source files, and so can `go generate`,
  `go vet -fix`, a test, and a program under `go run`.
- By R11, `go-comptime` writes the template and the generator again too.
- No build follows. To write a comptime file, `go-comptime` runs the analysis
  and the generator, so the errors of R1 and R3 appear. A build would add the
  compile errors of the whole package, and `go fmt` succeeds on code that does
  not compile.
- The requirement does not say what `go-comptime` does when it cannot write a
  comptime file from the changed code, or when a source file is deleted or
  loses its last tagged variable.
- Without this requirement the comptime file is stale. With a stale comptime
  file written by hand, a traceback through the overlay gave `p.go:15`, the
  line of the panic before `go fmt`. Plain `go run .` and a comptime file
  written again both gave `p.go:13` (Go 1.27.4).

### R13. A build never reads a stale comptime file

Approved 2026-11-27.

**Requirement.** When `go-comptime` is invoked with a subcommand of R7,
`go-comptime` shall write the comptime file of each source file that changed
since its comptime file was written, before it runs the subcommand.

**Example.**

```go
package main

import "fmt"

//qudy:comptime
func Mask(width int64) uint64 { return 1<<uint64(width) - 1 }

//qudy:comptime
var byteMask = Mask(8)

func main() { fmt.Println(byteMask) }
```

**Output and behavior.**

```
$ go-comptime run .
255
```

An editor then changes `Mask(8)` to `Mask(4)` in `p.go`.

```
$ go-comptime run .
15
```

Before the second run, `go-comptime` writes `.comptime/p.go` again, now with
`uint64(0xf)`.

**Notes.**

- Without this requirement, the second run prints 255. The overlay makes the
  build read the old comptime file, so the edit has no effect.
- R12 covers a source file that a subcommand of `go-comptime` changed. This
  requirement covers every other change, such as an edit in an editor or a
  plain `go fmt`.
- The requirement does not say how `go-comptime` finds out that a source file
  changed. Writing every comptime file again on every run satisfies it.
- R6 covers a source file that has no comptime file yet.
- The output above comes from comptime files written by hand and
  `go run -overlay` on Go 1.27.4. With the stale file the run printed 255;
  plain `go run .` and the file written again printed 15.
