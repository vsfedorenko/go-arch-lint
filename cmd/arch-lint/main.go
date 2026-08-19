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

	"github.com/vsfedorenko/go-arch-lint/internal/app"
)

const (
	flagProjectPath = "--project-path"
	flagBaseline    = "--baseline"
	flagHelp        = "--help"
)

// `go run` does not propagate the child's exit code: any non-zero child exit
// becomes go run's exit 1, with the original code mentioned only in the last
// stderr line ("exit status N"). The launcher parses it back so the
// linter-conventional codes (0 OK / 1 violations / 2 config error) survive
// the delegation. Stderr lines that are NOT "exit status N" are build/delegation
// failures, which map to config-error (2).
var childExitStatusRe = regexp.MustCompile(`^exit status (\d+)$`)

// childExitStatus extracts the child process exit code from `go run`'s
// stderr ("exit status N" as its last line). ok reports whether the child
// actually ran: absent means `go run` failed earlier — build errors,
// module resolution, syntax errors in the spec.
func childExitStatus(stderr string) (code int, ok bool) {
	for _, line := range strings.Split(stderr, "\n") {
		if m := childExitStatusRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if parsed, parseErr := strconv.Atoi(m[1]); parseErr == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

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

	if code, ran := childExitStatus(stderr); ran {
		return code
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
		fmt.Printf("go-arch-lint launcher %s (commit %s, built %s)\n", app.Version, app.CommitHash, app.BuildTime)
		return 0
	case "init":
		return cmdInit(os.Args[2:])
	case "help", flagHelp, "-h":
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

	delegatedArgs := make([]string, 0, len(args)+4)
	projectPathSet := false
	for i := 0; i < len(args); i++ {
		isPathFlag := args[i] == flagProjectPath || args[i] == "-p"
		hasValue := i+1 < len(args)

		switch {
		case isPathFlag && hasValue:
			delegatedArgs = append(delegatedArgs, args[i], absProjectPath)
			projectPathSet = true
			i++
		case args[i] == flagBaseline && hasValue:
			// The delegated process runs with cwd=.go-arch-lint/, so a
			// relative baseline path would resolve against the wrong
			// directory — absolutize it against the user's cwd first
			// (same treatment as --project-path).
			absBaseline, err := filepath.Abs(args[i+1])
			if err != nil {
				absBaseline = args[i+1]
			}
			delegatedArgs = append(delegatedArgs, args[i], absBaseline)
			i++
		default:
			delegatedArgs = append(delegatedArgs, args[i])
		}
	}
	if !projectPathSet {
		delegatedArgs = append(delegatedArgs, flagProjectPath, absProjectPath)
	}

	// .go-arch-lint/ has its own go.mod; -C runs go from that directory.
	goArgs := append([]string{"-C", archDir, "run", ".", command}, delegatedArgs...)
	cmd := exec.Command("go", goArgs...) //nolint:gosec,noctx // intentional: CLI delegates to 'go run .go-arch-lint/' per documented design; signal propagation is handled by the foreground process group
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

		// The "did not build" footer belongs ONLY to the case where the
		// delegated process never started: `go run` failed at build/module
		// resolution, so no "exit status N" line exists. When the child DID
		// run and exited 2, the config error is already printed by the
		// renderer ("Error: ...") — adding the footer here blamed a
		// compiling spec (seen with the new --output-json-one-line
		// validation and the pre-existing --baseline-update one).
		if _, ran := childExitStatus(stderrBuf.String()); !ran {
			exitErr := &exec.ExitError{}
			if errors.As(runErr, &exitErr) {
				fmt.Fprintf(os.Stderr, "---\nThe arch spec at %s did not build.\n", filepath.Join(archDir, "arch.go"))
				fmt.Fprintf(os.Stderr, "Fix the compile errors above, or regenerate the scaffold with 'go-arch-lint init'.\n")
			}
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
	init          Create .go-arch-lint/ scaffold (go.mod, arch.go + main.go)
	              init --recipe <clean|hexagonal|ddd> starts from a known pattern
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
  -p string               short form of --project-path
  --max-warnings int      fail when more than N violations (default 512)
  --format string         check output format [text, json, sarif, junit, github-actions, html] — 'json' emits a
                          flat array of violations for CI pipelines, 'sarif' a SARIF 2.1.0 log for
                          GitHub Code Scanning, 'junit' a JUnit XML report for CI dashboards,
                          'github-actions' workflow commands (::error/::notice PR annotations),
                          'html' a standalone HTML report (self-contained, archive-friendly)
                          (default "text")
  --output-color          use ANSI colors in terminal output (default true)
  --output-color=false    same as --no-colors (cobra-style value form)
  --baseline string       baseline file with known violations; only NEW violations
                          fail the check (incremental adoption for legacy codebases)
  --baseline-update       record the current violations as the baseline (with --baseline)
  --no-colors             disable ANSI colors
  --output-type string    command output type [ascii, json] (default "ascii")
  --json                  alias for --output-type=json
  --output-json-one-line  JSON as single line payload (json output type only)

Exit codes (check):
  0   no violations
  1   architecture violations found
  2   configuration/system error (invalid spec, project unreadable)
`)
}
