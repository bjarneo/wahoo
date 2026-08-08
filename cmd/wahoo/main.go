package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bjarneo/wahoo/internal/scaffold"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "wahoo:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "new":
		return newProject(args[1:])
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func newProject(args []string) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	module := flags.String("module", "", "Go module path")
	local := flags.Bool("local", false, "use the current Wahoo checkout as a Go module replacement")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: wahoo new [--module path] [--local] <directory>")
	}
	directory, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	if *module == "" {
		*module = "example.com/" + scaffold.ProjectName(filepath.Base(directory))
	}
	frameworkPath := ""
	if *local {
		frameworkPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve framework path: %w", err)
		}
	}
	if err := scaffold.Create(directory, *module, frameworkPath); err != nil {
		return err
	}
	fmt.Printf("created %s\n", directory)
	fmt.Println("next steps:")
	fmt.Printf("  cd %s\n", flags.Arg(0))
	fmt.Println("  npm install --prefix web")
	fmt.Println("  npm run dev --prefix web")
	return nil
}

func usageError() error {
	printUsage()
	return errors.New("a command is required")
}

func printUsage() {
	fmt.Println("Wahoo, Go + React + Tailwind SaaS framework")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  wahoo new [--module path] [--local] <directory>")
	fmt.Println("  wahoo version")
}
