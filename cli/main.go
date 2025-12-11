package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/keybase/msgpackzip"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Printf("Input file path required.\nUsage: go run main.go <input.mpack>\n")
		os.Exit(3)
	}

	inputPath := args[0]
	in, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Unable to read input file: %v\n", err)
		os.Exit(3)
	}
	out, err := msgpackzip.Compress(in)
	if err != nil {
		fmt.Printf("Unable to compress: %v\n", err)
		os.Exit(3)
	}
	outputPath := fmt.Sprintf("%s.mpzip", inputPath)
	fmt.Printf("Outputting compressed data to %s\n", outputPath)
	if err := os.WriteFile(outputPath, out, 0644); err != nil {
		fmt.Printf("Unable to write output: %v\n", err)
		os.Exit(3)
	}
	fmt.Printf("Success. Input size: %d, output size: %d\n", len(in), len(out))
}
