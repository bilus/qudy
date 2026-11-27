# gen

gen generates Go programs for testing a comptime analyzer, and labels every
function it writes.

    p := gen.Example(7, gen.Default)   // the program of seed 7
    p.Write(dir)                       // a module: go.mod, main.go, p0/p0.go, ...
    for _, s := range p.Symbols { ... } // each function with its label

Inside a rapid test, `gen.Generate(t, cfg)` draws a program and rapid shrinks a
failing one.

## How a program is built

A state machine (`rapid.T.Repeat`) builds the program. Its actions open a
package, declare a struct or interface type, open a function, method, generic
function or recursive function, and add a statement to the open function. Each
action checks its precondition against the state, so every program type-checks.

Loops have literal bounds or stop after eight iterations, and recursion stops
after three calls, so every program terminates. An estimate of the cost of one
call keeps each function below 20,000 operations.

## Labels

| Label | Meaning |
| --- | --- |
| `Comptime` | Built only from the comptime fragment, the trusted symbols, and other `Comptime` functions. |
| `Impure` | Has a planted impurity. The rest of its body is comptime and cannot panic. |
| `Unknown` | Refers to a function that is not `Comptime`, perhaps on a path that never runs. |

An analyzer must never list an `Impure` function as comptime, and should list
every `Comptime` one. `Unknown` functions take no part in either assertion.

A `Comptime` function may panic. It then panics with the same value on every
call, because a statement holds at most one operation that can panic.

## Configurations

`gen.Default` and `gen.Pure` use every construct of the fragment, with and
without impurities. `gen.Core` and `gen.CorePure` leave out function values,
generic functions, interfaces, `append` and the trusted symbols. An analyzer
with a small rule set should list every `Comptime` function of a core program.

## Trusted symbols

`gen.Index()` lists the functions of the standard library that the generator
takes to be comptime without proof, such as `strings.ToUpper` and
`math/bits.OnesCount64`. A symbol of another module that is not in the index
counts as an impurity. `catalog.go` lists the impurities and the indirections
that can hide them.

## Tests

    go test -short .   # type-checks programs and checks the labels against go/types
    go test .          # also builds programs for three targets and runs them

`COMPTIME_PROGRAMS=160 go test -run LabelsHoldAtRunTime -v .` runs more
programs. `COMPTIME_SAMPLE=7 go test -run Sample -v .` prints one.
`COMPTIME_SHRINK=1 go test -run ShrinkDemo -v .` shows a shrunk counterexample
and fails by design.

One seed gives one program for fixed values of the flags `-rapid.steps` and
`-short`, which set the number of actions in a round.
