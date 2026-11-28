# Reading database rows into structs (strict)

## Problem

A package that owns its own row types wants the column list, the query and
the scan of each type, all from the `db` tags:

```go
type User struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Notes string // not a column
}
```

- `columns[User]()` should be `[]string{"id", "name", "email"}`. A column
  that appears twice is a mistake that should stop the build, not the program.
- `selectAll[User]("users")` should be the string
  `"SELECT id, name, email FROM users"`.
- `scan[User](rows)` should call `rows.Scan(&x.ID, &x.Name, &x.Email)`
  without reflection.

The package uses these functions only on its own types. Nothing outside it
calls them.

## Solution

> **Strict comptime, not opportunistic.** The source does not build without
> `go-comptime`; the generated file does. This follows
> [the strict proposal](../proposal-alternative-strict-comptime.md), which is
> not approved.

The comptime source works on types as values:

```go
// columns returns the db columns of T, in field order. A repeated column panics.
func columns[T any]() []string {
	var cols []string
	seen := map[string]bool{}
	for f := range comptime.TypeFor[T]().Fields() {
		col := f.Tag.Get("db")
		if col == "" {
			continue
		}
		if seen[col] {
			panic("store: column " + col + " appears twice in " + f.Name)
		}
		seen[col] = true
		cols = append(cols, col)
	}
	return cols
}

// selectAll returns the query that reads every column of T from table.
func selectAll[T any](table comptime.Const[string]) string {
	return "SELECT " + strings.Join(columns[T](), ", ") + " FROM " + table.Get()
}

// scanner is what *sql.Rows offers for reading one row.
type scanner interface{ Scan(dest ...any) error }

// scan reads the current row into a new T.
func scan[T any](rows scanner) (T, error) {
	var x T
	v := comptime.ValueOf(&x)
	var dest []any
	for f := range comptime.TypeFor[T]().Fields() {
		if f.Tag.Get("db") != "" {
			dest = append(dest, v.Field(f).Addr().Interface())
		}
	}
	err := rows.Scan(dest...)
	return x, err
}
```

- `columns[User]()` and `selectAll[User](comptime.Lit("users"))` have every
  argument known at build time, so they become literals.
- `scan[User]` has a run-time argument, so it is specialized: the loop over the
  fields is unrolled and each `Addr().Interface()` becomes `&x.Field`.
- A repeated column panics during evaluation, which strict comptime reports as
  an error at the source line, the way Zig's `@compileError` does.

The generated code, for a `users` function that queries the table:

```go
func scan_User(rows *sql.Rows) (User, error) {
	var x User
	err := rows.Scan(&x.ID, &x.Name, &x.Email)
	return x, err
}

func users(db *sql.DB) ([]User, error) {
	rows, err := db.Query("SELECT id, name, email FROM users")
	…
}
```

## Why there is also an opportunistic version

Strict comptime functions do not exist at run time. A library therefore
cannot export `scan` for its callers' types: a caller without `go-comptime`
could not build, and a caller with it would need the source of `scan`. Callers
cannot use `scan` on their own types, which is why the exported version of
this problem is the opportunistic one in
[rowscan-opportunistic.md](rowscan-opportunistic.md).

## Evidence

- With the `comptime` package running on `reflect`, `selectAll[User]`
  returns `"SELECT id, name, email FROM users"`, and `scan[User]` reads a fake
  row into `User{ID: 7, Name: "Ann", Email: "ann@example.com"}`.
- `columns` on a struct with two fields tagged `db:"x"` panics.
- The generated code is hand-written; no tool produced it.
