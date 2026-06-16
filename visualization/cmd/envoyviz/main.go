package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/demo/envoyviz"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "parse":
		return runParse(args[1:])
	case "diff":
		return runDiff(args[1:])
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func runParse(args []string) int {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	recursive := fs.Bool("recursive", false, "scan subdirectories when path is a folder")
	format := fs.String("format", "", "input format for stdin: json or yaml")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: envoyviz parse [--recursive] [--format json|yaml] <file|folder|->")
		return 2
	}

	cfg, err := envoyviz.ParsePath(fs.Arg(0), envoyviz.ParseOptions{
		Recursive: *recursive,
		Format:    *format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	fmt.Println(envoyviz.FormatConfig(cfg))
	return 0
}

func runDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	recursive := fs.Bool("recursive", false, "scan subdirectories when path is a folder")
	noColor := fs.Bool("no-color", false, "disable ANSI colors")
	leftLabel := fs.String("left", "", "label for left column")
	rightLabel := fs.String("right", "", "label for right column")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: envoyviz diff [--recursive] [--no-color] [--left name] [--right name] <file|folder> <file|folder>")
		return 2
	}

	left, err := envoyviz.ParsePath(fs.Arg(0), envoyviz.ParseOptions{Recursive: *recursive})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse left: %v\n", err)
		return 1
	}
	right, err := envoyviz.ParsePath(fs.Arg(1), envoyviz.ParseOptions{Recursive: *recursive})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse right: %v\n", err)
		return 1
	}

	result := envoyviz.Compare(left, right)
	envoyviz.RenderDiff(os.Stdout, left, right, result, envoyviz.RenderOptions{
		NoColor:    *noColor,
		LeftLabel:  *leftLabel,
		RightLabel: *rightLabel,
	})

	if result.HasDiffs() {
		return 1
	}
	return 0
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envoyviz parse  [--recursive] [--format json|yaml] <file|folder|->")
	fmt.Fprintln(w, "  envoyviz diff   [--recursive] [--no-color] [--left name] [--right name] <file|folder> <file|folder>")
}
