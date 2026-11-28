# Problems

Each file here states a problem that `go-comptime` should solve. It then
gives the solution reached in design discussion, in a separate section. None
of these files is a requirement. A problem leads to a requirement only once
the project owner approves that requirement's wording in
[requirements.md](../requirements.md).

A problem has two files when its opportunistic and strict solutions differ in
code or in API. Opportunistic means the source builds and runs without
`go-comptime`, as [README.md](../README.md) describes. Strict means the design
of [proposal-alternative-strict-comptime.md](../proposal-alternative-strict-comptime.md),
which is not approved. When a solution is not opportunistic, its section
begins with a notice that says so.

| Problem | Opportunistic | Strict |
| --- | --- | --- |
| Reading database rows into structs | [rowscan-opportunistic.md](rowscan-opportunistic.md) | [rowscan-strict.md](rowscan-strict.md) |
| Checking struct fields against `validate` tags | [validator-opportunistic.md](validator-opportunistic.md) | [validator-strict.md](validator-strict.md) |
| A base64 encoding from a literal alphabet | [b64.md](b64.md) | — |
| A variadic `...any` function | [variadic.md](variadic.md) | — |
| An array type of computed length | — | [vector.md](vector.md) |
| A named type whose definition is computed | — | [geo.md](geo.md) |
| A struct type derived from another | — | [patchof.md](patchof.md) |

`go-comptime` does not exist yet. Every generated file below was written by
hand as the output `go-comptime` must produce. Unless a file says otherwise,
its checks ran on Go 1.27.
