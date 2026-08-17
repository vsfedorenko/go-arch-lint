package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const flagProjectPath = "--project-path"

// `go run` does not propagate the child's exit code: any non-zero child exit
// becomes go run's exit 1, with the original code mentioned only in the last
// stderr line ("exit status N"). The launcher parses it back so the
// linter-conventional codes (0 OK / 1 violations / 2 config error) survive
// the delegation. Stderr lines that are NOT "exit status N" are build/delegation
// failures, which map to config-error (2).
var childExitStatusRe = regexp.MustCompile(`^exit status (\d+)$`)

// delegatedExitCode maps a `go run` invocation result to the launcher's exit
// code. stderr is the delegated process's captured stderr.
func delegatedExitCode(err error, stderr string) int {
	if err == nil {
		return 0
	}

	exitErr := &exec.ExitError{}
	if !errors.As(err, &exitErr) {
		// go binary missing, signal, etc. — system error
		return 2
	}

	for _, line := range strings.Split(stderr, "\n") {
		if m := childExitStatusRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if code, parseErr := strconv.Atoi(m[1]); parseErr == nil {
				return code
			}
		}
	}

	// Non-zero exit without an "exit status N" line: the spec failed to
	// compile or `go run` itself failed (module resolution, syntax errors).
	// That is a configuration error, not an architecture violation.
	return 2
}

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
		return cmdInit(os.Args[2:])
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
		if (a == flagProjectPath || a == "-p") && i+1 < len(args) {
			projectPath = args[i+1]
			break
		}
	}

	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve project path '%s': %v\n", projectPath, err)
		return 1
	}

	archDir := filepath.Join(absProjectPath, ".go-arch-lint")
	if !dirExists(archDir) {
		fmt.Fprintf(os.Stderr, "Error: .go-arch-lint/ directory not found at %s\n", archDir)
		fmt.Fprintf(os.Stderr, "Run 'go-arch-lint init' first to create your arch config.\n")
		return 1
	}

	delegatedArgs := make([]string, 0, len(args)+2)
	projectPathSet := false
	for i := 0; i < len(args); i++ {
		if (args[i] == flagProjectPath || args[i] == "-p") && i+1 < len(args) {
			delegatedArgs = append(delegatedArgs, args[i], absProjectPath)
			projectPathSet = true
			i++
		} else {
			delegatedArgs = append(delegatedArgs, args[i])
		}
	}
	if !projectPathSet {
		delegatedArgs = append(delegatedArgs, flagProjectPath, absProjectPath)
	}

	// .go-arch-lint/ has its own go.mod; -C runs go from that directory.
	goArgs := append([]string{"-C", archDir, "run", ".", command}, delegatedArgs...)
	cmd := exec.Command("go", goArgs...) //nolint:gosec // intentional: CLI delegates to 'go run .go-arch-lint/' per documented design
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	// stderr is teed: shown to the user AND captured, because `go run` only
	// reports the child's exit code via its last stderr line ("exit status N").
	var stderrBuf strings.Builder
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	runErr := cmd.Run()
	code := delegatedExitCode(runErr, stderrBuf.String())

	// Friendly hints for the two system-level failure modes that otherwise
	// print nothing (go missing) or raw compiler noise without context
	// (spec does not compile).
	if runErr != nil {
		var execErr *exec.Error
		if errors.As(runErr, &execErr) && errors.Is(runErr, exec.ErrNotFound) {
			fmt.Fprintf(os.Stderr, "Error: 'go' is not on PATH — go-arch-lint delegates to 'go run .go-arch-lint/'.\n")
			fmt.Fprintf(os.Stderr, "Install Go from https://go.dev/dl/ and ensure 'go' is executable.\n")
			return code
		}

		exitErr := &exec.ExitError{}
		if errors.As(runErr, &exitErr) && code == 2 {
			fmt.Fprintf(os.Stderr, "---\nThe arch spec at %s did not build.\n", filepath.Join(archDir, "main.go"))
			fmt.Fprintf(os.Stderr, "Fix the compile errors above, or regenerate the scaffold with 'go-arch-lint init'.\n")
		}
	}

	return code
}

func dirExists(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // intentional: user-provided project path is the tool's purpose
	return err == nil && info.IsDir()
}

func printUsage() {
	fmt.Print(`go-arch-lint v2.0 — Go architectural linter

Usage:
  go-arch-lint <command> [flags]

Commands:
	init          Create .go-arch-lint/ scaffold (go.mod, main.go)
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
  --format string         check output format [text, json, sarif] — 'json' emits a flat
                          array of violations for CI pipelines, 'sarif' emits a
                          SARIF 2.1.0 log for GitHub Code Scanning (default "text")
  --output-color          use ANSI colors (default true)

Exit codes (check):
  0   no violations
  1   architecture violations found
  2   configuration/system error (invalid spec, project unreadable)
`)
}
