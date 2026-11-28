# Specification of "Opportunistic comptime for Go"

- [requirements.md](requirements.md): the approved requirements.
- [comptime-rules.md](comptime-rules.md): the comptime types, and the rules
  by which the analyzer derives that code is comptime.
- [vocabulary.md](vocabulary.md): the terms the requirements use.
- [limitations.md](limitations.md): the cases the requirements do not reach.

This directory holds only approved requirements. A proposed requirement enters
after the project owner approves its wording. It then gets the next number,
R1, R2 and so on, and no number is used twice.

`go-comptime` does not exist yet, so each example states the output that it
must print.

## Two layers

`requirements.md` has one section for each layer, and every requirement
belongs to one of them:

- **Binding-time analysis** decides whether an expression is comptime, and
  when tagged code is an error.
- **The driver, `go-comptime`** writes files, runs the `go` command, and
  settles which positions Go reports.

A number records the order of approval, so the numbers in a section can have
gaps.

## How to read a requirement

Each requirement has four parts:

- **Requirement:** one sentence in EARS form. No keyword for what always
  holds, "When" for an event, "While" for a state, "Where" for an optional feature,
  and "If ... then" for unwanted behavior.
- **Example:** a complete Go file, named `p.go`. A requirement about the
  driver also shows the files that `go-comptime` writes.
- **Output and behavior:** what `go-comptime` prints and does for that file.
- **Notes:** optional.

## Scope of the first version

Code is run-time unless a fact says that it is comptime, and in this version
every fact comes from a `//qudy:comptime` directive. The analyzer looks for a
derivation of each such fact and infers no fact of its own. `go-comptime`
evaluates only the initialization expressions of tagged variables.
