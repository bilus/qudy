// Command enums writes two files for each enum, as one txtar archive.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

//go:generate go run ../../cmd/qudy -o write_gen.go write.go

func main() {
	pkg := flag.String("package", "main", "the package of the generated files")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: enums [-package name] Type=Constant,Constant...")
		os.Exit(2)
	}
	for _, arg := range flag.Args() {
		typ, constants, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintln(os.Stderr, "enums: want Type=Constant,Constant, got", arg)
			os.Exit(2)
		}
		names := strings.Split(constants, ",")
		file := strings.ToLower(typ)
		writeString(file, *pkg, typ, names)
		writeParse(file, *pkg, typ, names)
	}
}
