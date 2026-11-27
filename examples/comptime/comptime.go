// Package comptime lists the exported symbols of Go packages that a build-time program can evaluate ahead of time.
//
// A value is comptime when a program run at build time on the host computes
// exactly what the original program would have computed at run time on the
// target. A function is comptime when a call with comptime arguments gives a
// comptime result and has no effect anything else can observe.
//
// The package has two halves. The plumbing loads packages, orders them, solves
// recursion, carries a verdict per symbol from a package to its importers and
// prints the result. The [Rules] decide what is comptime; they are an
// allow-list, and [FirstRules] is the provisional first set.
//
// The analyzer lists a symbol as comptime only when a rule proves it. A
// construct that no rule names makes the symbol not comptime.
package comptime

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// Config says where to look for packages and which rules judge them.
type Config struct {
	Dir   string   // the directory to resolve patterns in; "" is the current one
	Env   []string // the environment of the go command; nil is that of the process
	Rules Rules    // the rules; nil is FirstRules
}

// Report is the result of an analysis, which holds for one GOOS and GOARCH.
type Report struct {
	GOOS     string     `json:"goos"`
	GOARCH   string     `json:"goarch"`
	Packages []*Package `json:"packages"` // sorted by path
}

// Package lists the exported symbols of one package.
type Package struct {
	Path    string   `json:"path"`
	Error   string   `json:"error,omitempty"` // why the package was not analyzed
	Symbols []Symbol `json:"symbols"`         // sorted by name
}

// Symbol is an exported symbol and its verdict.
type Symbol struct {
	Name string `json:"name"` // Name, or Type.Method for a method
	Kind Kind   `json:"kind"`
	Pos  string `json:"pos"`
	Verdict
}

// Analyze judges the exported symbols of the packages the patterns name.
//
// It loads the packages and their dependencies from source and runs the
// analysis over the import graph, dependencies first.
func Analyze(cfg Config, patterns ...string) (*Report, error) {
	rules := cfg.Rules
	if rules == nil {
		rules = FirstRules{}
	}
	report, err := target(cfg)
	if err != nil {
		return nil, err
	}
	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Dir: cfg.Dir, Env: cfg.Env}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("the packages cannot be loaded: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, errors.New("the patterns match no packages")
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{NewAnalyzer(rules)}, pkgs, nil)
	if err != nil {
		return nil, fmt.Errorf("the analysis cannot run: %w", err)
	}
	for _, act := range graph.Roots {
		report.Packages = append(report.Packages, packageOf(act))
	}
	slices.SortFunc(report.Packages, func(a, b *Package) int { return strings.Compare(a.Path, b.Path) })
	return report, nil
}

// packageOf returns the result of one analysis action, or the error that prevented it.
//
// A package with any error is not reported, even when the analysis ran: a
// declaration the loader could not read might be the one that matters.
func packageOf(act *checker.Action) *Package {
	pkg, ok := act.Result.(*Package)
	if ok && act.Err == nil && len(act.Package.Errors) == 0 {
		return pkg
	}
	pkg = &Package{Path: act.Package.PkgPath, Symbols: []Symbol{}}
	switch {
	case len(act.Package.Errors) > 0:
		pkg.Error = act.Package.Errors[0].Error()
	case act.Err != nil:
		pkg.Error = act.Err.Error()
	default:
		pkg.Error = "the analysis gave no result"
	}
	return pkg
}

// target asks the go command which GOOS and GOARCH it builds for.
func target(cfg Config) (*Report, error) {
	cmd := exec.Command("go", "env", "GOOS", "GOARCH")
	cmd.Dir, cmd.Env = cfg.Dir, cfg.Env
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("the go command cannot tell the target: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return nil, fmt.Errorf("the go command gave an unexpected target: %q", out)
	}
	return &Report{GOOS: fields[0], GOARCH: fields[1]}, nil
}
