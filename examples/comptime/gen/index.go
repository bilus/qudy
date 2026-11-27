package gen

import "sort"

// external is a function of another module that the comptime fragment may call.
type external struct {
	symbol   string // the import path, a dot, and the name
	path     string // the import path
	call     string // the call, with one %s for each parameter
	params   []*typ
	result   *typ
	nonconst bool // an int64 argument is a variable, because a constant may not convert
}

// index lists the trusted symbols: functions of the standard library that the
// generator takes to be comptime without proof. Each returns for every argument
// the fragment can pass, and a slice it returns is newly allocated. A symbol of
// another module that is not listed here counts as an impurity.
var index = []external{
	{symbol: "strings.ToUpper", path: "strings", call: "strings.ToUpper(%s)", params: []*typ{tString}, result: tString},
	{symbol: "strings.ToLower", path: "strings", call: "strings.ToLower(%s)", params: []*typ{tString}, result: tString},
	{symbol: "strings.TrimSpace", path: "strings", call: "strings.TrimSpace(%s)", params: []*typ{tString}, result: tString},
	{symbol: "strings.ReplaceAll", path: "strings", call: "strings.ReplaceAll(%s, %s, %s)", params: []*typ{tString, tString, tString}, result: tString},
	{symbol: "strings.Join", path: "strings", call: "strings.Join(%s, %s)", params: []*typ{tStrings, tString}, result: tString},
	{symbol: "strconv.FormatInt", path: "strconv", call: "strconv.FormatInt(%s, 10)", params: []*typ{tInt}, result: tString},
	{symbol: "strconv.Quote", path: "strconv", call: "strconv.Quote(%s)", params: []*typ{tString}, result: tString},
	{symbol: "path.Base", path: "path", call: "path.Base(%s)", params: []*typ{tString}, result: tString},
	{symbol: "strings.Contains", path: "strings", call: "strings.Contains(%s, %s)", params: []*typ{tString, tString}, result: tBool},
	{symbol: "strings.HasPrefix", path: "strings", call: "strings.HasPrefix(%s, %s)", params: []*typ{tString, tString}, result: tBool},
	{symbol: "strings.EqualFold", path: "strings", call: "strings.EqualFold(%s, %s)", params: []*typ{tString, tString}, result: tBool},
	{symbol: "unicode/utf8.ValidString", path: "unicode/utf8", call: "utf8.ValidString(%s)", params: []*typ{tString}, result: tBool},
	{symbol: "unicode.IsUpper", path: "unicode", call: "unicode.IsUpper(rune(%s))", params: []*typ{tInt}, result: tBool, nonconst: true},
	{symbol: "slices.Contains", path: "slices", call: "slices.Contains(%s, %s)", params: []*typ{tInts, tInt}, result: tBool},
	{symbol: "strings.Index", path: "strings", call: "int64(strings.Index(%s, %s))", params: []*typ{tString, tString}, result: tInt},
	{symbol: "strings.Count", path: "strings", call: "int64(strings.Count(%s, %s))", params: []*typ{tString, tString}, result: tInt},
	{symbol: "strings.Compare", path: "strings", call: "int64(strings.Compare(%s, %s))", params: []*typ{tString, tString}, result: tInt},
	{symbol: "unicode/utf8.RuneCountInString", path: "unicode/utf8", call: "int64(utf8.RuneCountInString(%s))", params: []*typ{tString}, result: tInt},
	{symbol: "math/bits.OnesCount64", path: "math/bits", call: "int64(bits.OnesCount64(uint64(%s)))", params: []*typ{tInt}, result: tInt, nonconst: true},
	{symbol: "math/bits.Len64", path: "math/bits", call: "int64(bits.Len64(uint64(%s)))", params: []*typ{tInt}, result: tInt, nonconst: true},
	{symbol: "math/bits.Reverse64", path: "math/bits", call: "int64(bits.Reverse64(uint64(%s)))", params: []*typ{tInt}, result: tInt, nonconst: true},
	{symbol: "slices.Index", path: "slices", call: "int64(slices.Index(%s, %s))", params: []*typ{tStrings, tString}, result: tInt},
	{symbol: "unicode.ToUpper", path: "unicode", call: "int64(unicode.ToUpper(rune(%s)))", params: []*typ{tInt}, result: tInt, nonconst: true},
	{symbol: "strings.Fields", path: "strings", call: "strings.Fields(%s)", params: []*typ{tString}, result: tStrings},
	{symbol: "strings.Split", path: "strings", call: "strings.Split(%s, %s)", params: []*typ{tString, tString}, result: tStrings},
	{symbol: "slices.Clone", path: "slices", call: "slices.Clone(%s)", params: []*typ{tInts}, result: tInts},
}

// Index returns the trusted symbols, sorted. Each is an import path, a dot, and a name.
func Index() []string {
	symbols := make([]string, len(index))
	for i, e := range index {
		symbols[i] = e.symbol
	}
	sort.Strings(symbols)
	return symbols
}
