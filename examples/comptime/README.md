# comptime

An experimental, conservative static analyzer for "Opportunistic comptime for
Go". It lists the exported symbols of Go packages that a build-time program can
evaluate ahead of time.

    go run ./examples/comptime/cmd/comptime strings      # the comptime symbols
    go run ./examples/comptime/cmd/comptime -stats std   # counts and reasons
    go run ./examples/comptime/cmd/comptime -json ./...  # every symbol, with its reason

A symbol is listed only when a rule proves it comptime. The rules are an
allow-list: a construct that no rule names makes the symbol not comptime, and
the output says which construct and where. A verdict can carry a condition,
such as `int-size`: it holds when host and target agree on the size of `int`.

## Layout

- `comptime.go`, `analyzer.go`: loading, the order of packages, recursion, and
  the verdict each package hands to its importers (a `go/analysis` fact).
- `rules.go`: the interface between that plumbing and the rules.
- `first.go`, `first_body.go`: `FirstRules`, the provisional first rule set.
- `verdict.go`, `output.go`: verdicts, reasons, conditions, text and JSON.
- `gen/`: a generator of test programs with labelled functions; see its README.
- `spec/`: the approved requirements and the vocabulary; see its README.

## Tests

    go test -short ./examples/comptime/...   # small test packages, one per reason
    go test ./examples/comptime/...          # also the standard library and generated programs

`TestGeneratedPrograms` checks the analyzer against the generator: no function
with a planted impurity may be listed, and the log says how many of the
functions that are comptime by construction are.

`TestCorePrograms` does the same with core programs, which use only constructs
that the first rules cover, and requires that every comptime function is listed.

`TestDisabledRules` shows that these tests can fail. It disables one rule at a
time and requires that a function with a planted impurity is then listed.
