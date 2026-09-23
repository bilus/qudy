package qudy

// gensymVariable is the name of the symbol generator in a function body.
const gensymVariable = "qudyGensym"

// scopeVariable names the function that starts a scope of the generated program.
const scopeVariable = "qudyScope"

// gensymSuffix ends every name that qudy makes for a gensym.
const gensymSuffix = "_qd"

// gensymName returns the generator's variable for the gensym of a name.
func gensymName(name string) string {
	return name + gensymSuffix
}

// gensymBody counts gensyms and writes each count in decimal, without an import.
const gensymBody = `count := 0
	gensym := func(want string) string {
		count++
		digits := ""
		for n := count; n > 0; n /= 10 {
			digits = string(rune('0'+n%10)) + digits
		}
		return want + "` + gensymSuffix + `" + digits
	}`

// gensymDecl declares the built-in symbol generator.
const gensymDecl = gensymVariable + ` := func() func(string) string {
	` + gensymBody + `
	return gensym
}()`

// gensymScopeDecl declares the built-in symbol generator and its scope function.
const gensymScopeDecl = gensymVariable + `, ` + scopeVariable + ` := func() (func(string) string, func() func()) {
	` + gensymBody + `
	return gensym, func() func() {
		saved := count
		return func() { count = saved }
	}
}()`
