package qudy

// gensymVariable is the name of the symbol generator in a function body.
const gensymVariable = "qudyGensym"

// gensymSuffix ends every name that qudy makes for a gensym.
const gensymSuffix = "_qd"

// gensymName returns the generator's variable for the gensym of a name.
func gensymName(name string) string {
	return name + gensymSuffix
}

// gensymDecl is the declaration of the built-in symbol generator.
const gensymDecl = gensymVariable + ` := func() func(string) string {
	counts := map[string]int{}
	return func(want string) string {
		counts[want]++
		digits := ""
		for n := counts[want]; n > 0; n /= 10 {
			digits = string(rune('0'+n%10)) + digits
		}
		return want + "` + gensymSuffix + `" + digits
	}
}()`
