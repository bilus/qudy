# Limitations

Each entry names a case in which `go-comptime` cannot do what the
requirements promise, and says why.

## `go test -cover` tests the source files

With `-cover` or `-coverprofile`, `go test` ignores the overlay and compiles
the source files, not the comptime files (Go 1.27.4). The tests then check the
code without comptime.
