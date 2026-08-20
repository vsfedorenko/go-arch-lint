package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const scaffoldGoMod = `module arch-lint-local

go 1.25
`

// scaffoldArchGo holds the user-editable spec. It lives in its own file so
// the runner (main.go) can be regenerated or upgraded without touching the
// architecture description.
//
// The ExcludeFiles line is assembled from an interpreted string: "\\." renders
// as the single backslash "\." in the written file — a regex escape for the
// dot. (A historical version wrote "\\\\.", which rendered a literal backslash
// that matched no real file names.)
const scaffoldArchGo = `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	// Define your components:
	// Component("handler", "handlers/*")
	// Component("service", "services/**")

	// Define dependency rules:
	// Deps("handler", func() {
	//     MayDependOn("service")
	// })
})
`

// scaffoldMainGo is the stable runner. Keep user-facing configuration in
// arch.go; this file only forwards CLI flags and executes the spec.
// MustRunCLI (not MustRun) so delegated commands (mapping, graph,
// selfInspect) keep their own behavior instead of silently degrading
// to a check run.
const scaffoldMainGo = `package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v2"
)

func main() {
	archlint.MustRunCLI(spec, os.Args[1:])
}
`

// recipeFlag is the flag selecting a starter spec: init --recipe <name>.
const recipeFlag = "--recipe"

// parseInitArgs extracts --project-path/-p (space and = forms) and --recipe
// (space and = forms) from init's args. A flag present without its value is
// an error — silently scaffolding the DEFAULT spec when the user asked for a
// recipe (but typo'd the invocation) writes the wrong starting point.
func parseInitArgs(args []string) (projectPath, recipe string, err error) {
	projectPath = "."
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--project-path" || a == "-p":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value (the project directory)", a)
			}
			projectPath = args[i+1]
			i++
		case strings.HasPrefix(a, "--project-path="):
			projectPath = strings.TrimPrefix(a, "--project-path=")
		case a == recipeFlag:
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value; pick one of the names below", recipeFlag)
			}
			recipe = args[i+1]
			i++
		case strings.HasPrefix(a, recipeFlag+"="):
			recipe = strings.TrimPrefix(a, recipeFlag+"=")
		}
	}
	return projectPath, recipe, nil
}

// recipeHelp renders the known recipes for error/usage output.
func recipeHelp() string {
	names := make([]string, 0, len(knownRecipes))
	for name := range knownRecipes {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("Available recipes:\n")
	for _, name := range names {
		fmt.Fprintf(&b, "  %-12s %s\n", name, knownRecipes[name].desc)
	}
	return b.String()
}

// printInitUsage explains init's flags without touching the filesystem —
// `init --help` previously scaffolded a project instead of showing help.
func printInitUsage() {
	fmt.Print(`go-arch-lint init — create the .go-arch-lint/ scaffold

Usage:
  go-arch-lint init [flags]

Flags:
  --recipe string        starter spec for a known pattern
  -p, --project-path     project directory (default "./")

` + recipeHelp())
}

func cmdInit(args []string) int {
	for _, a := range args {
		if a == flagHelp || a == "-h" {
			printInitUsage()
			return 0
		}
	}

	projectPath, recipe, err := parseInitArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, recipeHelp())
		return 1
	}

	if recipe != "" {
		archGo, ok := recipeArchGo(recipe)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: unknown recipe '%s'\n\n%s", recipe, recipeHelp())
			return 1
		}
		return writeScaffold(projectPath, archGo, fmt.Sprintf(" (%s recipe)", recipe))
	}

	return writeScaffold(projectPath, scaffoldArchGo, "")
}

// writeScaffold creates the .go-arch-lint directory with go.mod, arch.go and
// main.go. archGo is the spec body (plain scaffold or a recipe); label is
// appended to the final "scaffold created" line, may be empty.
func writeScaffold(projectPath, archGo, label string) int {
	archDir := filepath.Join(projectPath, ".go-arch-lint")

	if dirExists(archDir) {
		fmt.Fprintf(os.Stderr, "Error: %s already exists\n", archDir)
		return 1
	}

	if err := os.MkdirAll(archDir, 0o755); err != nil { //nolint:gosec // intentional: creates scaffold dir at user-specified path
		fmt.Fprintf(os.Stderr, "Error: failed to create %s: %v\n", archDir, err)
		return 1
	}

	files := map[string]string{
		"go.mod":  scaffoldGoMod,
		"arch.go": archGo,
		"main.go": scaffoldMainGo,
	}

	for name, content := range files {
		path := filepath.Join(archDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // intentional: generated source files use standard 0644 permissions
			fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
			return 1
		}
		fmt.Printf("  created %s\n", path)
	}

	fmt.Printf("\nScaffold created%s. Next steps:\n", label)
	fmt.Printf("  1. Edit %s/arch.go to describe your architecture\n", archDir)
	fmt.Printf("  2. Run 'cd %s && go mod tidy' to resolve the github.com/vsfedorenko/go-arch-lint/v2 dependency\n", archDir)
	fmt.Printf("  3. Run 'go-arch-lint check' to lint your project\n")

	return 0
}
