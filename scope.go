package qudy

// Scope names the generator's supply of fresh names.
type Scope struct {
	variable string
	create   string
	method   string
}

// NewScope returns a scope, such as NewScope("sc", "newScope()", "fresh").
func NewScope(variable, create, method string) Scope {
	return Scope{variable: variable, create: create, method: method}
}

// decl returns the statement that declares the scope.
func (s Scope) decl() string {
	return s.variable + " := " + s.create
}

// call returns the function that hands out a name.
func (s Scope) call() string {
	return s.variable + "." + s.method
}

// fresh returns the call that hands out the given name.
func (s Scope) fresh(name string) string {
	return s.call() + `("` + name + `")`
}

// empty reports whether the scope is the zero value.
func (s Scope) empty() bool {
	return s == Scope{}
}
