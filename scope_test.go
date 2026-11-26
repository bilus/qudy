package qudy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bilus/qudy"
)

// runAlone compiles a template without a runtime file and runs the generator as one file.
func runAlone(t *testing.T, src string) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not on the path")
	}
	gen, err := qudy.Compile("main.go", []byte(src), "fmt.Printf")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "main_gen.go")
	if err := os.WriteFile(file, gen, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), goTool, "run", file).CombinedOutput()
	if err != nil {
		t.Fatalf("the generator fails: %v\n%s", err, out)
	}
	return string(out)
}

// branches assigns a gensym in one branch and uses it in a later branch, in text and in braces.
const branches = `package main

func main() {
	for _, single := range []bool{true, false} {
		end := qudyScope()
		if single {
			//` + "`" + `value# := read()
		} else {
			//` + "`" + `value#, err# := read()
			//` + "`" + `check(err#)
		}
		if single {
			//` + "`" + `set(~{wrap(value#)})
		} else {
			//` + "`" + `set(value#)
		}
		//` + "`" + `done(value#)
		end()
	}
}

func wrap(name string) string { return "wrap(" + name + ")" }
`

// nestedFirst uses a gensym in a block first, then outside it and in a loop.
const nestedFirst = `package main

func main() {
	if len("a") == 1 {
		//` + "`" + `a# := 1
	}
	//` + "`" + `use(a#)
	for range 2 {
		//` + "`" + `again(a#)
	}
}
`

// loops writes a gensym in a loop without a scope, then two in a loop with a scope per iteration.
const loops = `package main

func main() {
	for range 2 {
		//` + "`" + `shared# := 0
	}
	for i := range 2 {
		end := qudyScope()
		if i == 0 {
			//` + "`" + `a# b#
		} else {
			//` + "`" + `b# a#
		}
		end()
	}
}
`

// nestedScope uses two gensyms before a scope, inside it in the other order, and after it.
const nestedScope = `package main

func main() {
	//` + "`" + `a# b#
	end := qudyScope()
	//` + "`" + `b# a#
	end()
	//` + "`" + `a# b#
}
`

// helperTwice calls a function with a gensym twice inside the scope of its caller.
const helperTwice = `package main

func main() {
	defer qudyScope()()
	//` + "`" + `a#
	helper()
	helper()
	//` + "`" + `a#
}

func helper() {
	if len("a") == 1 {
		//` + "`" + `a# \
	}
	//` + "`" + `a#
}
`

// perIteration tries four ways to a new name in every iteration of a loop without a scope.
const perIteration = `package main

func main() {
	for range 2 {
		//` + "`" + `plain# \
		func() {
			//` + "`" + `literal# \
		}()
		called()
		direct := qudyGensym("direct")
		//` + "`" + `~direct
	}
}

func called() {
	//` + "`" + `called# \
}
`

func TestAScopeGivesAGensymOneName(t *testing.T) {
	type testCase struct {
		description string
		template    string
		alone       string
		withRuntime string
	}
	same := func(want string) (string, string) { return want, want }
	branchesAlone, branchesRuntime := same("value_qd1 := read()\nset(wrap(value_qd1))\ndone(value_qd1)\n" +
		"value_qd1, err_qd2 := read()\ncheck(err_qd2)\nset(value_qd1)\ndone(value_qd1)\n")
	nestedAlone, nestedRuntime := same("a_qd1 := 1\nuse(a_qd1)\nagain(a_qd1)\nagain(a_qd1)\n")
	loopsAlone, loopsRuntime := same("shared_qd1 := 0\nshared_qd1 := 0\na_qd2 b_qd3\nb_qd2 a_qd3\n")
	scopeAlone, scopeRuntime := same("a_qd1 b_qd2\nb_qd3 a_qd4\na_qd1 b_qd2\n")
	for _, c := range []testCase{
		{"in two branches side by side and a later line", branches, branchesAlone, branchesRuntime},
		{"in a block first and then outside it", nestedFirst, nestedAlone, nestedRuntime},
		{"in every iteration of a loop, unless the iteration starts a scope", loops, loopsAlone, loopsRuntime},
		{"but a scope inside the scope has names of its own", nestedScope, scopeAlone, scopeRuntime},
		{
			"but every call of a function has names of its own",
			helperTwice,
			"a_qd1\na_qd1 a_qd1\na_qd1 a_qd1\na_qd1\n",
			"a_qd1\na_qd2 a_qd2\na_qd3 a_qd3\na_qd1\n",
		},
		{
			"and in a loop a qudyGensym call alone makes a new name in both modes",
			perIteration,
			"plain_qd1 literal_qd2 called_qd1 direct_qd3\nplain_qd1 literal_qd2 called_qd1 direct_qd4\n",
			"plain_qd1 literal_qd2 called_qd3 direct_qd4\nplain_qd1 literal_qd2 called_qd5 direct_qd6\n",
		},
	} {
		t.Run(c.description, func(t *testing.T) {
			if got := runAlone(t, c.template); got != c.alone {
				t.Errorf("without a runtime file the generator wrote %q, want %q", got, c.alone)
			}
			got := runWithRuntime(t, "fmt.Printf", map[string]string{"main.go": c.template}, nil)
			if got != c.withRuntime {
				t.Errorf("with a runtime file the generator wrote %q, want %q", got, c.withRuntime)
			}
		})
	}
}

func TestCompileRejectsAGensymUnderALocalScopeFunction(t *testing.T) {
	src := template("qudyGensym, qudyScope := seeded(taken)", "defer qudyScope()()", "//`a := errName#")
	_, err := qudy.Compile("test.go", []byte(src), "g.generate")
	if want := "test.go:5: a gensym in a function that declares qudyScope as a local is not supported"; err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Compile returned %v, want a rejection naming %q", err, want)
	}
}

// ownScope declares qudyScope at package level and passes it to a function as a parameter.
const ownScope = `package main

var depth int

func qudyScope() func() {
	depth++
	return func() { depth-- }
}

func main() {
	//` + "`" + `a# \
	end := qudyScope()
	//` + "`" + `a# ~depth \
	end()
	//` + "`" + `a# ~depth
	param(qudyScope)
}

func param(qudyScope func() func()) {
	//` + "`" + `b# \
	end := qudyScope()
	//` + "`" + `b#
	end()
}
`

func TestAScopeFunctionOfTheTemplateStartsNewNames(t *testing.T) {
	if got, want := runAlone(t, ownScope), "a_qd1 a_qd2 1 a_qd1 0\nb_qd1 b_qd2\n"; got != want {
		t.Errorf("the generator wrote %q, want %q", got, want)
	}
}
