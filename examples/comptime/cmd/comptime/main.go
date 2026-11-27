// Command comptime lists the exported symbols of Go packages that can be evaluated at build time.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bilus/qudy/examples/comptime"
)

func main() {
	asJSON := flag.Bool("json", false, "print every exported symbol with its verdict and reason, as JSON")
	stats := flag.Bool("stats", false, "print summary statistics instead of the symbols")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
		os.Exit(2)
	}
	report, err := comptime.Analyze(comptime.Config{}, flag.Args()...)
	if err == nil {
		err = write(report, *asJSON, *stats)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "comptime:", err)
		os.Exit(1)
	}
	if failed := report.Stats().Failed; failed > 0 {
		fmt.Fprintf(os.Stderr, "comptime: %d packages were not analyzed\n", failed)
		os.Exit(1)
	}
}

// usage prints how to call the command.
func usage() {
	fmt.Fprintln(os.Stderr, "usage: comptime [-json] [-stats] packages...")
	flag.PrintDefaults()
}

// write prints the report or its statistics, as text or as JSON.
func write(report *comptime.Report, asJSON, stats bool) error {
	switch {
	case asJSON && stats:
		return comptime.WriteJSON(os.Stdout, report.Stats())
	case asJSON:
		return comptime.WriteJSON(os.Stdout, report)
	case stats:
		return report.Stats().WriteText(os.Stdout)
	}
	return report.WriteText(os.Stdout)
}
