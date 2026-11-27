package gen

// kind classifies the types a generated program computes with.
type kind int

const (
	kInt kind = iota
	kBool
	kString
	kSlice
	kMap
	kStruct
	kPointer
	kFunc
	kIface
	kParam
	kPick // a func(T) int64, the second parameter of a generic function
)

// typ is a type of the generated program. Equal types are the same pointer.
type typ struct {
	kind kind
	elem *typ      // of a slice or a pointer
	decl *typeDecl // of a struct, a pointer to a struct, or an interface type
}

// The types that need no declaration. A map is a map[string]int64, a function
// a func(int64) int64, and tParam the one type parameter of a generic function.
var (
	tInt     = &typ{kind: kInt}
	tBool    = &typ{kind: kBool}
	tString  = &typ{kind: kString}
	tInts    = &typ{kind: kSlice, elem: tInt}
	tStrings = &typ{kind: kSlice, elem: tString}
	tMap     = &typ{kind: kMap}
	tFunc    = &typ{kind: kFunc}
	tParam   = &typ{kind: kParam}
	tPick    = &typ{kind: kPick}
)

// basics are the types of struct fields, slice elements and literals.
var basics = []*typ{tInt, tBool, tString}

// basic reports whether the type is int64, bool or string.
func (t *typ) basic() bool { return t.kind <= kString }

// shared reports whether two values of the type can refer to the same variable.
func (t *typ) shared() bool { return t.kind == kSlice || t.kind == kMap || t.kind == kPointer }

// field is a field of a generated struct type.
type field struct {
	name string
	typ  *typ
}

// typeDecl is a generated struct or interface type. Every generated type is exported.
type typeDecl struct {
	pkg     *pkg
	name    string
	fields  []field         // of a struct; each has a basic type, so a struct value shares nothing
	method  string          // of an interface: its one method, a func(int64) int64
	impls   []*fn           // of an interface: the finished methods that implement it
	fns     []*fn           // of a struct: its finished methods
	methods map[string]bool // of a struct: the interface methods it has
	typ     *typ
	ptr     *typ // of a struct: the pointer to it
}

// pkg is a package of the generated program.
type pkg struct {
	name, path string
	imports    map[string]bool
	decls      []string // finished declarations, in order
	types      []*typeDecl
	funcs      []*fn // finished functions and methods, in order
	clip       bool  // the package needs the clip helper
}

// fn is a generated function or method.
type fn struct {
	pkg        *pkg
	name       string
	kind       string    // "func", "method", "generic", "recursive" or "main"
	recv       *typeDecl // of a method
	ptrRecv    bool      // the receiver is a pointer
	recvName   string
	implements *typeDecl // of a method that implements an interface
	params     []variable
	result     *typ
	exported   bool
	partner    *fn  // of a mutually recursive pair: the other function
	second     bool // of a pair: this function is written second
	limit      float64
	cost       float64 // an estimate of the operations one call executes
	loose      bool    // the body may refer to functions that are not comptime
	calm       bool    // the body must not panic
	panics     bool    // a call may panic
	planted    bool    // an impurity is planted in the body
	taint      string  // the variable that holds the planted impure value
	impurity   string
	shows      string
	layers     []string
	refs       []*fn // the functions the body refers to
	reach      bool  // the body refers to an impurity, directly or through refs
	features   map[string]bool
}

// symbol returns the function's name as a Symbol reports it.
func (f *fn) symbol() string {
	if f.recv != nil {
		return f.recv.name + "." + f.name
	}
	return f.name
}

// label returns what the generator knows about the function.
func (f *fn) label() Label {
	switch {
	case f.planted:
		return Impure
	case f.reach:
		return Unknown
	}
	return Comptime
}

// variable is a local variable, parameter or receiver in scope.
type variable struct {
	name       string
	typ        *typ
	assignable bool
	owned      bool // the function created what the value refers to, and may write to it
	unique     bool // no other variable refers to the same slice, so append is predictable
	param      bool // a parameter or a pointer receiver: its value comes from the caller
}
