package gen

import "fmt"

// impurity is one way for a function's result to stop being comptime.
type impurity struct {
	name    string
	imports []string
	decls   string // package-level declarations
	body    string // the body of func impure() int64
	between string // what main does between its two rounds of calls
}

// impurities is the catalog. Each entry is one reason a value is not comptime.
var impurities = []impurity{
	{name: "clock", imports: []string{"time"}, body: "return time.Now().UnixNano()"},
	{name: "process id", imports: []string{"os"}, body: "return int64(os.Getpid())"},
	{name: "environment", imports: []string{"os"}, body: `return int64(len(os.Getenv("HOME")))`},
	{name: "random number", imports: []string{"math/rand/v2"}, body: "return rand.Int64()"},
	{name: "write of a package variable", decls: "var counter int64\n\n", body: "counter++\n\treturn counter"},
	{name: "read of a package variable", decls: "var state int64\n\n", body: "return state", between: "state = 7"},
	{name: "map iteration order", body: "for k := range map[int64]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true} {\n\t\treturn k\n\t}\n\treturn 0"},
	{name: "select with two ready cases", body: "a, b := make(chan int64, 1), make(chan int64, 1)\n\ta <- 1\n\tb <- 2\n\tselect {\n\tcase x := <-a:\n\t\treturn x\n\tcase x := <-b:\n\t\treturn x\n\t}"},
	{name: "goroutine scheduling", imports: []string{"sync"}, body: "var mu sync.Mutex\n\tvar wg sync.WaitGroup\n\tvar r int64\n\tfor i := int64(1); i <= 3; i++ {\n\t\twg.Add(1)\n\t\tgo func() {\n\t\t\tdefer wg.Done()\n\t\t\tmu.Lock()\n\t\t\tr = r*10 + i\n\t\t\tmu.Unlock()\n\t\t}()\n\t}\n\twg.Wait()\n\treturn r"},
	{name: "size of int", body: "one := 1\n\treturn int64(one << 40)"},
	{name: "address of a variable", imports: []string{"unsafe"}, body: "return int64(uintptr(unsafe.Pointer(new(int64))))"},
	{name: "fused multiply-add", imports: []string{"math"}, decls: "//go:noinline\nfunc mulAdd(x, y, z float64) float64 { return x*y + z }\n\n", body: "return int64(math.Float64bits(mulAdd(0.1, 10, -1)))"},
}

// layer is one indirection between a caller and the function it hides.
type layer struct {
	name    string
	imports []string
	wrap    func(i int, prev string) (decls, name string)
}

// layers are the ways to hide a call from an analysis that follows only static calls.
var layers = []layer{
	{name: "static call", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 { return %s() }\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "function value", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 {\n\tf := %s\n\treturn f()\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "interface method", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("type i%d interface{ get() int64 }\n\ntype s%d struct{}\n\nfunc (s%d) get() int64 { return %s() }\n\nfunc h%d() int64 {\n\tvar x i%d = s%d{}\n\treturn x.get()\n}\n\n", i, i, i, prev, i, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "function literal", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 {\n\tvar r int64\n\tfunc() { r = %s() }()\n\treturn r\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "deferred call", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() (r int64) {\n\tdefer func() { r = %s() }()\n\treturn 0\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "generic function", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func g%d[T any](f func() T) T { return f() }\n\nfunc h%d() int64 { return g%d(%s) }\n\n", i, i, i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "method value", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("type m%d struct{}\n\nfunc (m%d) get() int64 { return %s() }\n\nfunc h%d() int64 {\n\tg := m%d{}.get\n\treturn g()\n}\n\n", i, i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "package variable of function type", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("var f%d = %s\n\nfunc h%d() int64 { return f%d() }\n\n", i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "package variable initializer", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("var init%d = %s()\n\nfunc h%d() int64 { return init%d }\n\n", i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "reflect call", imports: []string{"reflect"}, wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 { return reflect.ValueOf(%s).Call(nil)[0].Int() }\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
}
