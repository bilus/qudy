package qudy

// SymbolGenerator is the variable, create expression and method of the generator's symbol generator.
type SymbolGenerator struct {
	variable string
	create   string
	method   string
}

// NewSymbolGenerator returns a symbol generator, such as NewSymbolGenerator("symbols", "newSymbolGenerator()", "gensym").
func NewSymbolGenerator(variable, create, method string) SymbolGenerator {
	return SymbolGenerator{variable: variable, create: create, method: method}
}

// decl returns the short variable declaration of the symbol generator.
func (s SymbolGenerator) decl() string {
	return s.variable + " := " + s.create
}

// methodValue returns the symbol generator's method value, such as symbols.gensym.
func (s SymbolGenerator) methodValue() string {
	return s.variable + "." + s.method
}

// callFor returns the call to the symbol generator's method with want as its argument.
func (s SymbolGenerator) callFor(want string) string {
	return s.methodValue() + "(" + want + ")"
}

// isZero reports whether the symbol generator is the zero value.
func (s SymbolGenerator) isZero() bool {
	return s == SymbolGenerator{}
}
