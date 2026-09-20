//go:build go1.27

// Command sumtype generates the boilerplate for a package's sum types.
package main

import (
	"fmt"
	"os"

	"github.com/bilus/qudy/examples/sumtype"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sumtype <package directory>")
		os.Exit(2)
	}
	written, err := sumtype.Generate(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sumtype:", err)
		os.Exit(1)
	}
	for _, path := range written {
		fmt.Println(path)
	}
}
