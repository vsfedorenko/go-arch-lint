package app

import (
	"context"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/app/container"
)

// Flag spellings shared by the translator and its tests. Kept as
// constants so the launcher dialect stays greppable and goconst-clean.
const (
	flagLongProjectPath = "--project-path"
	flagNoColors        = "--no-colors"
	flagOut             = "--out"
	cmdCheck            = "check"
)

// RunCLI executes the delegated CLI surface (check, mapping, graph,
// self-inspect, version, help) against an in-process spec. It is the
// programmatic twin of the launcher binary: the scaffolded
// `.go-arch-lint/main.go` calls it so that EVERY delegated command keeps
// its own behavior — previously the scaffold always ran `check`, and
// `mapping`/`graph`/`selfInspect` silently degraded to a check run.
//
// args is the process argument list WITHOUT the binary name (os.Args[1:]),
// including the command name as the first element. The returned error
// follows the archlint.Run contract: nil on success, a user-space error
// when the check found violations, anything else is a config/system
// error (map with archlint.ExitCode).
func RunCLI(ctx context.Context, specDecoder container.SpecDecoder, args []string) error {
	di := newContainer()
	err := di.RunCLI(ctx, specDecoder, TranslateLauncherArgs(args))
	reportSystemError(err)
	return err
}

// TranslateLauncherArgs adapts the launcher flag dialect to the cobra
// dialect used by the in-process command tree:
//
//   - short project-path     "-p <path>" / "-p=<path>" → "--project-path <path>"
//     (cobra registers no -p shorthand)
//   - "--no-colors"           → "--output-color=false"
//     (same meaning, the only spelling cobra knows)
//   - "selfInspect"           → "self-inspect"
//     (the launcher documents the camelCase name; the command tree
//     registers the kebab-case one)
//   - no command token at all → "check" is prepended
//     (bare `go run .go-arch-lint/` used to run a check through
//     archlint.MustRun; silently printing help instead would flip a
//     violations exit 1 into exit 0)
//
// The function is pure: same input, same output, no globals touched.
func TranslateLauncherArgs(args []string) []string {
	translated := make([]string, 0, len(args)+1)

	for i := 0; i < len(args); i++ {
		a := args[i]

		switch {
		case a == "-p" && i+1 < len(args):
			translated = append(translated, flagLongProjectPath, args[i+1])
			i++
		case strings.HasPrefix(a, "-p="):
			translated = append(translated, flagLongProjectPath+"="+strings.TrimPrefix(a, "-p="))
		case a == flagNoColors:
			translated = append(translated, "--output-color=false")
		case a == "selfInspect":
			translated = append(translated, "self-inspect")
		default:
			translated = append(translated, a)
		}
	}

	if commandName(translated) == "" {
		return append([]string{cmdCheck}, translated...)
	}

	return translated
}

// commandName returns the first non-flag token, skipping the values of
// value-taking flags ("--project-path <v>" etc. — the "=value" forms
// carry their value inline and consume no next token). Empty when the
// argument list carries no command.
func commandName(args []string) string {
	valueFlags := map[string]bool{
		flagLongProjectPath: true,
		"--arch-file":       true,
		flagOut:             true,
		"--focus":           true,
		"--baseline":        true,
		"--max-warnings":    true,
		"--scheme":          true,
		"--type":            true,
		"--format":          true,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return a
		}
		if strings.Contains(a, "=") {
			continue
		}
		if valueFlags[a] && i+1 < len(args) {
			i++
		}
	}

	return ""
}
