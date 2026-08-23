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

	"github.com/vsfedorenko/go-arch-lint/v3/internal/app"
	versionop "github.com/vsfedorenko/go-arch-lint/v3/internal/operations/version"
)

const (
	flagProjectPath = "--project-path"
	flagBaseline    = "--baseline"
	flagOut         = "--out"
	flagHelp        = "--help"

	// defaultGraphOutFile mirrors the graph command's own default output
	// file name (internal/app/container/container_cmd_graph.go).
	defaultGraphOutFile = "go-arch-lint-graph.svg"

	// cmdGraph is the delegated graph command name (see run's switch).
	cmdGraph = "graph"

	// versionLinePrefix starts every version line; shared by the build-info
	// path and the ldflags fallback so the two can never diverge.
	versionLinePrefix = "go-arch-lint launcher "
)

// splitPathFlag recognizes a path-carrying flag in both its forms:
// "--flag value" (value empty, caller consumes the next arg) and
// "--flag=value" (value carried inline). isPath reports whether the token
// is a path flag at all.
func splitPathFlag(token string) (name, value string, isPath bool) {
	for _, name := range []string{flagProjectPath, "-p", flagBaseline, flagOut} {
		if token == name {
			return name, "", true
		}
		if strings.HasPrefix(token, name+"=") {
			return name, strings.TrimPrefix(token, name+"="), true
		}
	}
	return "", "", false
}

// isFlagLike reports whether a token looks like a flag rather than a value;
// used to avoid swallowing the next flag when a path flag lost its value.
func isFlagLike(token string) bool {
	return strings.HasPrefix(token, "-")
}

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

// printVersion serves the `version` command and its flag forms. When the
// operation fails (no build info), it falls back to the ldflags defaults
// rather than erroring: a version query must never fail.
func printVersion() int {
	out, err := versionop.NewOperation(app.Version, app.BuildTime, app.CommitHash).Behave()
	if err != nil {
		fmt.Printf("%s%s (commit %s, built %s)\n", versionLinePrefix, app.Version, app.CommitHash, app.BuildTime)
		return 0
	}
	fmt.Printf("%s%s (commit %s, built %s)\n", versionLinePrefix, out.LinterVersion, out.CommitHash, out.BuildTime)
	return 0
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	command := os.Args[1]

	switch command {
	case "version":
		return printVersion()
	case "init":
		return cmdInit(os.Args[2:])
	case "help", flagHelp, "-h":
		printUsage()
		return 0
	default:
		// A flag-like first token is not a command: the documented
		// form is `go-arch-lint <command> [flags]`. Before this guard,
		// a leading flag was delegated as a command name and the user
		// got a misleading ".go-arch-lint/ directory not found" config
		// error outside a project (`--version` asked for a version and
		// was told to run init) or cobra's generic "unknown flag"
		// inside one. Serve the version flag forms here, fail fast
		// naming the token otherwise.
		if isFlagLike(command) {
			switch command {
			case "--version", "-v", "-V":
				return printVersion()
			}
			fmt.Fprintf(os.Stderr, "Error: unknown flag or command: %s\n", command)
			fmt.Fprintf(os.Stderr, "Run 'go-arch-lint help' for usage.\n")
			return 1
		}
		// All other commands (check, mapping, graph, selfInspect) delegate
		// to `go run .go-arch-lint/`
		return cmdDelegate(command, os.Args[2:])
	}
}

func cmdDelegate(command string, args []string) int {
	// --help must never require a project: a user exploring the tool outside
	// any project should get usage, not a config error (same class as
	// `init --help` not requiring a scaffold). Without .go-arch-lint/ there
	// is nothing to delegate `go run` to, so the launcher's own usage —
	// which already documents every delegated flag — is the best answer.
	for _, a := range args {
		if a == flagHelp || a == "-h" {
			printUsage()
			return 0
		}
	}

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
	outSet := false
	for i := 0; i < len(args); i++ {
		name, value, isPath := splitPathFlag(args[i])
		if !isPath {
			delegatedArgs = append(delegatedArgs, args[i])
			continue
		}

		// Path-carrying flags (--project-path, --baseline, --out) accept
		// both "--flag value" and "--flag=value". The delegated process
		// runs with cwd=.go-arch-lint/, so a relative path would resolve
		// against the wrong directory — absolutize it against the user's
		// cwd first. A missing value (last token or followed by another
		// flag) fails fast HERE: the launcher appends its own defaults
		// (--project-path, graph's --out), which would silently satisfy
		// the delegated parser and degrade to default behavior — the
		// exact silent no-op the flag contract forbids.
		if value == "" {
			if i+1 >= len(args) || isFlagLike(args[i+1]) {
				fmt.Fprintf(os.Stderr, "Error: flag needs an argument: %s\n", name)
				fmt.Fprintf(os.Stderr, "Run 'go-arch-lint help' for usage.\n")
				return 1
			}
			value = args[i+1]
			i++ // consume the value token of the space form
		}

		absValue, err := filepath.Abs(value)
		if err != nil {
			absValue = value
		}
		delegatedArgs = append(delegatedArgs, name+"="+absValue)

		switch name {
		case flagProjectPath, "-p":
			projectPathSet = true
		case flagOut:
			outSet = true
		}
	}
	if !projectPathSet {
		delegatedArgs = append(delegatedArgs, flagProjectPath+"="+absProjectPath)
	}
	if command == cmdGraph && !outSet {
		// The graph command's own default ("./go-arch-lint-graph.svg") would
		// resolve against the delegated cwd (.go-arch-lint/) — pin it to the
		// project root, exactly where an explicit --out lands.
		delegatedArgs = append(delegatedArgs, flagOut+"="+filepath.Join(absProjectPath, defaultGraphOutFile))
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
	fmt.Print(`go-arch-lint v3.0 — Go architectural linter

Usage:
  go-arch-lint <command> [flags]

Commands:
	init          Create .go-arch-lint/ scaffold (go.mod, arch.go + main.go)
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
