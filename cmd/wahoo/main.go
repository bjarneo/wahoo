package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
	case "add":
		return addModules(args[1:])
	case "modules":
		printModules()
		return nil
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
	modules := flags.String("modules", "", "comma-separated optional modules")
	nonInteractive := flags.Bool("yes", false, "create a core-only project without prompts")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: wahoo new [--module path] [--local] [--modules names] [--yes] <directory>")
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
	selected, err := scaffold.ParseModules([]string{*modules})
	if err != nil {
		return err
	}
	if *modules == "" && !*nonInteractive {
		selected, err = promptModules(os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
	}
	if err := scaffold.Create(directory, *module, frameworkPath, selected...); err != nil {
		return err
	}
	if err := tidyProject(directory); err != nil {
		return err
	}
	if *local {
		if err := vendorProject(directory); err != nil {
			return err
		}
	}
	fmt.Printf("created %s\n", directory)
	fmt.Println("next steps:")
	fmt.Printf("  cd %s\n", flags.Arg(0))
	fmt.Println("  npm ci --prefix web")
	fmt.Println("  npm run dev --prefix web")
	return nil
}

func addModules(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: wahoo add <project-directory> <module...>")
	}
	directory, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	modules, err := scaffold.ParseModules(args[1:])
	if err != nil {
		return err
	}
	if err := scaffold.Add(directory, modules...); err != nil {
		return err
	}
	if err := tidyProject(directory); err != nil {
		return err
	}
	usesLocalFramework, err := usesLocalFramework(directory)
	if err != nil {
		return err
	}
	if usesLocalFramework {
		if err := vendorProject(directory); err != nil {
			return err
		}
	}
	fmt.Printf("added modules to %s: %s\n", directory, joinModules(modules))
	return nil
}

func promptModules(in io.Reader, out io.Writer) ([]scaffold.Module, error) {
	reader := bufio.NewReader(in)
	questions := []struct {
		module scaffold.Module
		prompt string
	}{
		{module: scaffold.ModuleAuth, prompt: "Add authentication route stubs? [y/N]: "},
		{module: scaffold.ModuleSSE, prompt: "Add an SSE endpoint? [y/N]: "},
		{module: scaffold.ModuleWebSocket, prompt: "Add a WebSocket endpoint? [y/N]: "},
	}
	var modules []scaffold.Module
	for _, question := range questions {
		fmt.Fprint(out, question.prompt)
		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read module selection: %w", err)
		}
		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "", "n", "no":
		case "y", "yes":
			modules = append(modules, question.module)
		default:
			return nil, fmt.Errorf("invalid answer for %s", question.module)
		}
	}
	return modules, nil
}

func joinModules(modules []scaffold.Module) string {
	parts := make([]string, len(modules))
	for i, module := range modules {
		parts[i] = string(module)
	}
	return strings.Join(parts, ", ")
}

func tidyProject(directory string) error {
	return runGoModuleCommand(directory, "tidy")
}

func vendorProject(directory string) error {
	return runGoModuleCommand(directory, "vendor")
}

func runGoModuleCommand(directory, operation string) error {
	var commands [][]string
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err == nil {
		commands = append(commands, []string{goBin})
	}
	commands = append(commands, []string{"go"})
	if _, err := exec.LookPath("mise"); err == nil {
		commands = append(commands, []string{"mise", "exec", "go@1.25.12", "--", "go"})
	}

	var errs []error
	for _, command := range commands {
		args := append(command[1:], "mod", operation)
		cmd := exec.Command(command[0], args...)
		cmd.Dir = directory
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		if detail := strings.TrimSpace(string(output)); detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		errs = append(errs, err)
	}
	return fmt.Errorf("%s Go module: %w", operation, errors.Join(errs...))
}

func usesLocalFramework(directory string) (bool, error) {
	goMod, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		return false, fmt.Errorf("read Go module: %w", err)
	}
	return strings.Contains(string(goMod), "replace github.com/bjarneo/wahoo =>"), nil
}

func usageError() error {
	printUsage()
	return errors.New("a command is required")
}

func printUsage() {
	fmt.Println("Wahoo, Go + React + Tailwind SaaS framework")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  wahoo new [--module path] [--local] [--modules names] [--yes] <directory>")
	fmt.Println("  wahoo add <project-directory> <module...>")
	fmt.Println("  wahoo modules")
	fmt.Println("  wahoo version")
}

func printModules() {
	fmt.Println("Optional modules:")
	fmt.Println("  auth       Account route stubs")
	fmt.Println("  sse        Server-Sent Events endpoint")
	fmt.Println("  websocket  Development WebSocket endpoint")
}
