package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Printf("go-arch-lint launcher v2.0.0-dev\n")
		return 0
	case "init":
		return cmdInit()
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		// All other commands (check, mapping, graph, selfInspect) delegate
		// to `go run .go-arch-lint/`
		return cmdDelegate(command, os.Args[2:])
	}
}

func cmdDelegate(command string, args []string) int {
	projectPath := "."
	for i, a := range args {
		if (a == "--project-path" || a == "-p") && i+1 < len(args) {
			projectPath = args[i+1]
			break
		}
	}

	archDir := filepath.Join(projectPath, ".go-arch-lint")
	if !dirExists(archDir) {
		fmt.Fprintf(os.Stderr, "Error: .go-arch-lint/ directory not found at %s\n", archDir)
		fmt.Fprintf(os.Stderr, "Run 'go-arch-lint init' first to create your arch config.\n")
		return 1
	}

	// Build the command: go run .go-arch-lint/ <command> [args...]
	goArgs := append([]string{"run", archDir, command}, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Error: failed to run arch-lint: %v\n", err)
		return 1
	}

	return 0
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func printUsage() {
	fmt.Print(`go-arch-lint v2.0 — Go architectural linter

Usage:
  go-arch-lint <command> [flags]

Commands:
  init          Create .go-arch-lint/ scaffold (go.mod, main.go, arch.go)
  check         Check project architecture against arch rules
  mapping       Show package-to-component mapping
  graph         Generate dependency graph
  selfInspect   Inspect go-arch-lint's own architecture
  version       Print version
  help          Show this help

The 'check', 'mapping', 'graph', and 'selfInspect' commands require a
.go-arch-lint/ directory (created by 'init') and delegate to 'go run'.

Global flags (passed through to delegated commands):
  --project-path string   project directory (default "./")
  --output-type string    output format [ascii, json] (default "ascii")
  --json                  alias for --output-type=json
  --output-color          use ANSI colors (default true)
`)
}

