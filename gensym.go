package qudy

// gensymVariable is the name of the symbol generator in a function body.
const gensymVariable = "qudyGensym"

// scopeVariable names the function that starts a scope of the generated program.
const scopeVariable = "qudyScope"

// nameVariable names the function that looks a gensym up among the names of its scope.
const nameVariable = "qudyName"

// gensymSuffix ends every name that qudy makes for a gensym.
const gensymSuffix = "_qd"

// gensymName returns the generator's variable for the gensym of a name.
func gensymName(name string) string {
	return name + gensymSuffix
}

// nameLookup returns the name of a gensym in the scope's names, and makes it on first use.
func nameLookup(name string) string {
	return nameVariable + "(\"" + name + "\", " + gensymVariable + ")"
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

// nameBody keeps the names of the open scope; the caller passes the symbol
// generator, because a template may declare its own after this point.
const nameBody = `names := map[string]string{}
	name := func(want string, gensym func(string) string) string {
		found, ok := names[want]
		if !ok {
			found = gensym(want)
			names[want] = found
		}
		return found
	}`

// nameDecl declares the names of a function body without a scope call.
const nameDecl = nameVariable + ` := func() func(string, func(string) string) string {
	` + nameBody + `
	return name
}()`

// nameScopeDecl declares the names of a function body and a scope function that starts with none.
const nameScopeDecl = nameVariable + `, ` + scopeVariable + ` := func(scope func() func()) (func(string, func(string) string) string, func() func()) {
	` + nameBody + `
	return name, func() func() {
		outer := names
		names = map[string]string{}
		end := scope()
		return func() {
			end()
			names = outer
		}
	}
}(` + scopeVariable + `)`
