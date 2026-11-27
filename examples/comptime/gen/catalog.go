package gen

import "fmt"

// impurity is one way for a function's result to stop being comptime.
// In decls, body and between, a # stands for a number that makes names unique.
type impurity struct {
	name     string
	symbol   string // the symbol outside the index that it uses, if that is what makes it impure
	imports  []string
	decls    string // package-level declarations
	body     string // the body of a func() int64
	between  string // an assignment that main executes between its two rounds of calls
	shows    string // the condition that reliably changes the value: repeat, rerun, env, 386 or v3
	stateful bool   // one evaluation at package initialization gives a comptime value
}

// impurities is the catalog. Each entry is one reason a value is not comptime.
var impurities = []impurity{
	{name: "clock", symbol: "time.Now", imports: []string{"time"}, body: "return time.Now().UnixNano()", shows: "repeat"},
	{name: "random number", symbol: "math/rand/v2.Int64", imports: []string{"math/rand/v2"}, body: "return rand.Int64()", shows: "repeat"},
	{name: "process id", symbol: "os.Getpid", imports: []string{"os"}, body: "return int64(os.Getpid())", shows: "rerun"},
	{name: "environment", symbol: "os.Getenv", imports: []string{"os"}, body: `return int64(len(os.Getenv("HOME")))`, shows: "env"},
	{name: "write of a package variable", decls: "var counter# int64\n\n", body: "counter#++\nreturn counter#", shows: "repeat", stateful: true},
	{name: "read of a package variable", decls: "var State# int64\n\n", body: "return State#", between: "State# = 7", shows: "repeat", stateful: true},
	{name: "map iteration order", body: "m := map[int64]bool{}\nfor i := int64(0); i < 64; i++ {\nm[i] = true\n}\nvar r int64\nfor k := range m {\nr = r*31 + k\n}\nreturn r", shows: "repeat"},
	{name: "select with two ready cases", body: "var r int64\nfor range 62 {\na, b := make(chan int64, 1), make(chan int64, 1)\na <- 0\nb <- 1\nselect {\ncase x := <-a:\nr = r*2 + x\ncase x := <-b:\nr = r*2 + x\n}\n}\nreturn r", shows: "repeat"},
	{name: "goroutine scheduling", imports: []string{"sync"}, body: "var mu sync.Mutex\nvar wg sync.WaitGroup\nvar r int64\nfor i := int64(1); i <= 3; i++ {\nwg.Add(1)\ngo func() {\ndefer wg.Done()\nmu.Lock()\nr = r*10 + i\nmu.Unlock()\n}()\n}\nwg.Wait()\nreturn r"},
	{name: "size of int", body: "one := 1\nreturn int64(one << 40)", shows: "386"},
	{name: "address of a variable", symbol: "unsafe.Pointer", imports: []string{"unsafe"}, body: "return int64(uintptr(unsafe.Pointer(new(int64))))"},
	{name: "capacity chosen by append", body: "s := append([]string{\"a\"}, \"b\", \"c\", \"d\", \"e\")\nreturn int64(cap(s))", shows: "386"},
	{name: "append to an aliased slice", body: "s := []string{\"a\", \"b\", \"c\"}\ns = append(s, \"d\")\nu := s\ns = append(s, \"x\")\nu = append(u, \"yy\")\nreturn int64(len(s[4]) + len(u[4]))", shows: "386"},
	{name: "constant of the target", symbol: "runtime.GOARCH", imports: []string{"runtime"}, body: "return int64(len(runtime.GOARCH))", shows: "386"},
	{name: "fused multiply-add", symbol: "math.Float64bits", imports: []string{"math"}, decls: "//go:noinline\nfunc mulAdd#(x, y, z float64) float64 { return x*y + z }\n\n", body: "return int64(math.Float64bits(mulAdd#(0.1, 10, -1)))", shows: "v3"},
}

// parameterWrite names the impurity of a function that writes through a parameter.
const parameterWrite = "write through a parameter"

// layer is one indirection between a caller and the function it hides.
type layer struct {
	name    string
	imports []string
	wrap    func(i int, prev string) (decls, name string)
}

// initializerLayer evaluates the impurity once, when the package is initialized.
const initializerLayer = "package variable initializer"

// layers are the ways to hide a call from an analysis that follows only static calls.
var layers = []layer{
	{name: "static call", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 { return %s() }\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "function value", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 {\nf := %s\nreturn f()\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "interface method", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("type i%d interface{ get() int64 }\n\ntype s%d struct{}\n\nfunc (s%d) get() int64 { return %s() }\n\nfunc h%d() int64 {\nvar x i%d = s%d{}\nreturn x.get()\n}\n\n", i, i, i, prev, i, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "function literal", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 {\nvar r int64\nfunc() { r = %s() }()\nreturn r\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "deferred call", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() (r int64) {\ndefer func() { r = %s() }()\nreturn 0\n}\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "generic function", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func g%d[T any](f func() T) T { return f() }\n\nfunc h%d() int64 { return g%d(%s) }\n\n", i, i, i, prev), fmt.Sprintf("h%d", i)
	}},
	{name: "method value", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("type m%d struct{}\n\nfunc (m%d) get() int64 { return %s() }\n\nfunc h%d() int64 {\ng := m%d{}.get\nreturn g()\n}\n\n", i, i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "package variable of function type", wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("var f%d = %s\n\nfunc h%d() int64 { return f%d() }\n\n", i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: initializerLayer, wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("var init%d = %s()\n\nfunc h%d() int64 { return init%d }\n\n", i, prev, i, i), fmt.Sprintf("h%d", i)
	}},
	{name: "reflect call", imports: []string{"reflect"}, wrap: func(i int, prev string) (string, string) {
		return fmt.Sprintf("func h%d() int64 { return reflect.ValueOf(%s).Call(nil)[0].Int() }\n\n", i, prev), fmt.Sprintf("h%d", i)
	}},
}
