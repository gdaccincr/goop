// Command goopc translates goop class declarations into ordinary Go source.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gdaccincr/goop/transpiler"
)

func main() {
	inputPath := flag.String("in", "", "input source file")
	outputPath := flag.String("out", "", "output Go file (defaults to stdout)")
	runtimePath := flag.String("runtime", "", "goop runtime import path")
	flag.Parse()
	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "goopc: -in is required")
		os.Exit(2)
	}

	source, err := os.ReadFile(*inputPath)
	if err != nil {
		fatal(err)
	}
	generated, err := transpiler.CompileWithOptions(source, transpiler.Options{RuntimeImport: *runtimePath})
	if err != nil {
		fatal(err)
	}
	if *outputPath == "" || *outputPath == "-" {
		if _, err := os.Stdout.Write(generated); err != nil {
			fatal(err)
		}
		return
	}
	if err := os.WriteFile(*outputPath, generated, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
