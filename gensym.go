package qudy

// gensymVariable is the name of the symbol generator in a function body.
const gensymVariable = "qudyGensym"

// gensymDecl is the declaration of the built-in symbol generator.
const gensymDecl = gensymVariable + ` := func() func(string) string {
	counts := map[string]int{}
	return func(want string) string {
		counts[want]++
		digits := ""
		for n := counts[want]; n > 0; n /= 10 {
			digits = string(rune('0'+n%10)) + digits
		}
		return want + "_qd" + digits
	}
}()`
