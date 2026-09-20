//go:build go1.27

package sumtype

import "strconv"

// symbolGenerator is the set of taken names, and it returns gensyms outside that set.
type symbolGenerator struct {
	taken map[string]bool
}

// newSymbolGenerator returns a symbol generator that avoids the given names.
func newSymbolGenerator(taken []string) *symbolGenerator {
	s := &symbolGenerator{taken: map[string]bool{}}
	for _, name := range taken {
		s.taken[name] = true
	}
	return s
}

// gensym returns want, or want with the first number that leaves it unused.
func (s *symbolGenerator) gensym(want string) string {
	name := want
	for n := 1; s.taken[name]; n++ {
		name = want + strconv.Itoa(n)
	}
	s.taken[name] = true
	return name
}
