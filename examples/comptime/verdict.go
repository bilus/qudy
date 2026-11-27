package comptime

import (
	"slices"
	"strings"
)

// Kind names the sort of exported symbol a verdict is about.
type Kind string

// The kinds of symbol the analyzer reports.
const (
	KindConst  Kind = "const"
	KindVar    Kind = "var"
	KindFunc   Kind = "func"
	KindMethod Kind = "method"
)

// Condition is an assumption about the target under which a verdict holds.
type Condition string

// The conditions a verdict can carry.
const (
	// SameIntSize holds when host and target agree on the size of int, uint and uintptr.
	SameIntSize Condition = "int-size"
	// SameBuildConfig holds when host and target select the same files: the same GOOS, GOARCH and build tags.
	SameBuildConfig Condition = "build-config"
)

// ReasonKind names a class of construct that keeps a symbol from being comptime.
type ReasonKind string

// The reasons a symbol is not comptime.
const (
	// ReasonVariable: the symbol is a package-level variable, or reads or writes one.
	ReasonVariable ReasonKind = "package-variable"
	// ReasonNoBody: the function has no Go body (assembly, //go:linkname, cgo).
	ReasonNoBody ReasonKind = "no-body"
	// ReasonCgo: the symbol is declared in a file that cgo generated.
	ReasonCgo ReasonKind = "cgo"
	// ReasonDependency: the symbol calls a function, or uses a constant or a type, that is not comptime.
	ReasonDependency ReasonKind = "dependency"
	// ReasonDynamicCall: the function calls through an interface or a function value.
	ReasonDynamicCall ReasonKind = "dynamic-call"
	// ReasonFuncValue: the function makes a closure or uses a function as a value.
	ReasonFuncValue ReasonKind = "function-value"
	// ReasonForeignWrite: the function writes to memory it did not allocate.
	ReasonForeignWrite ReasonKind = "foreign-write"
	// ReasonConcurrency: the function uses go, select or a channel.
	ReasonConcurrency ReasonKind = "concurrency"
	// ReasonRecover: the function calls recover.
	ReasonRecover ReasonKind = "recover"
	// ReasonMapRange: the function ranges over a map.
	ReasonMapRange ReasonKind = "map-range"
	// ReasonUnsafe: the symbol uses package unsafe.
	ReasonUnsafe ReasonKind = "unsafe"
	// ReasonFloat: the function computes with floating-point or complex numbers.
	ReasonFloat ReasonKind = "floating-point"
	// ReasonUnspecified: the function observes something the language leaves to the implementation.
	ReasonUnspecified ReasonKind = "unspecified-behavior"
	// ReasonGeneric: the function has type parameters.
	ReasonGeneric ReasonKind = "generic"
	// ReasonUnsupported: the function uses a construct no rule names.
	ReasonUnsupported ReasonKind = "unsupported"
	// ReasonUnknown: no verdict was recorded for the symbol.
	ReasonUnknown ReasonKind = "unknown"
)

// ReasonKinds returns every reason kind, in the order of their declaration.
func ReasonKinds() []ReasonKind {
	return []ReasonKind{
		ReasonVariable, ReasonNoBody, ReasonCgo, ReasonDependency, ReasonDynamicCall, ReasonFuncValue,
		ReasonForeignWrite, ReasonConcurrency, ReasonRecover, ReasonMapRange, ReasonUnsafe, ReasonFloat,
		ReasonUnspecified, ReasonGeneric, ReasonUnsupported, ReasonUnknown,
	}
}

// Reason says why a symbol is not comptime.
type Reason struct {
	Kind   ReasonKind `json:"kind"`
	Pos    string     `json:"pos"`              // where the construct is, as file:line:column
	Cause  string     `json:"cause,omitempty"`  // the symbol that caused it, as path.Name or path.Type.Method
	Detail string     `json:"detail,omitempty"` // the construct, in words
	Why    *Reason    `json:"why,omitempty"`    // the reason the cause is not comptime itself
}

// Root returns the last reason of the chain, the construct that started it.
func (r *Reason) Root() *Reason {
	for r.Why != nil {
		r = r.Why
	}
	return r
}

// Verdict says whether a symbol is comptime, and under which conditions or for what reason not.
type Verdict struct {
	Comptime   bool        `json:"comptime"`
	Conditions []Condition `json:"conditions,omitempty"` // sorted, and empty unless Comptime
	Reason     *Reason     `json:"reason,omitempty"`     // nil exactly when Comptime
}

// Accepted returns the verdict of a symbol that is comptime under the given conditions.
func Accepted(conditions ...Condition) Verdict {
	slices.Sort(conditions)
	return Verdict{Comptime: true, Conditions: slices.Compact(conditions)}
}

// Rejected returns the verdict of a symbol that is not comptime.
func Rejected(reason *Reason) Verdict {
	return Verdict{Reason: reason}
}

// equal reports whether two verdicts say the same thing.
func (v Verdict) equal(w Verdict) bool {
	return v.Comptime == w.Comptime && v.Reason == w.Reason && slices.Equal(v.Conditions, w.Conditions)
}

// String renders a verdict as "comptime", "comptime if int-size" or "not comptime: no-body".
func (v Verdict) String() string {
	switch {
	case !v.Comptime && v.Reason == nil:
		return "not comptime"
	case !v.Comptime:
		return "not comptime: " + string(v.Reason.Kind)
	case len(v.Conditions) == 0:
		return "comptime"
	}
	names := make([]string, len(v.Conditions))
	for i, c := range v.Conditions {
		names[i] = string(c)
	}
	return "comptime if " + strings.Join(names, ", ")
}
