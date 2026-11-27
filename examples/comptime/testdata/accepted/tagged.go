//go:build !comptime_never

package accepted

// Tagged is declared in a file that a build constraint selects.
const Tagged = "tagged"

// UsesTagged depends on that file through the constant.
func UsesTagged() string { return Tagged }
