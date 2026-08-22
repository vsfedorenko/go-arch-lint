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
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	// The whole module as one component keeps the fresh scaffold green.
	// Split it into real directories and add Use rules as your
	// architecture takes shape:
	//
	//     domain := Path("internal/domain")
	//     Path("internal/core", func() { Use(domain) })
	Path(".")
})
`

// scaffoldMainGo is the stable runner for v2 specs. Keep user-facing
// configuration in arch.go; this file only forwards CLI flags and executes
// the spec. MustRunCLI so delegated commands (mapping,
// graph, selfInspect) keep their own behavior instead of silently
// degrading to a check run.
const scaffoldMainGo = `package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v3"
)

func main() {
	archlint.MustRunCLI(build, os.Args[1:])
}`

// parseInitArgs extracts --project-path/-p (space and = forms) from init's
// args. A flag present without its value is an error — silently scaffolding
// at the wrong path writes the wrong starting point.
func parseInitArgs(args []string) (projectPath string, err error) {
	projectPath = "."
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--project-path" || a == "-p":
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value (the project directory)", a)
			}
			projectPath = args[i+1]
			i++
		case strings.HasPrefix(a, "--project-path="):
			projectPath = strings.TrimPrefix(a, "--project-path=")
		}
	}
	return projectPath, nil
}

// printInitUsage explains init's flags without touching the filesystem —
// `init --help` previously scaffolded a project instead of showing help.
func printInitUsage() {
	fmt.Print(`go-arch-lint init — create the .go-arch-lint/ scaffold

Usage:
  go-arch-lint init [flags]

Flags:
  -p, --project-path     project directory (default "./")
`)
}

// scanGoDirs lists every directory under root (relative, slash-separated,
// sorted, excluding .go-arch-lint, vendor, hidden dirs and testdata)
// that directly contains a .go file. The scaffold declares each as a
// Path() so the fresh spec satisfies the declare-everything rule.
func scanGoDirs(root string) ([]string, []string) { //nolint:gosec // root is the init --project-path argument, same trust level as every other init write
	var dirs, excludes []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error { //nolint:gosec // root is the init --project-path argument, same trust level as every other init write
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped
		}
		if !d.IsDir() {
			return nil
		}
		if p == root {
			// the module root itself: declare as "." when it has Go files
			entries, readErr := os.ReadDir(p)
			if readErr != nil {
				return nil //nolint:nilerr // unreadable root: nothing to declare
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
					dirs = append(dirs, ".")
					break
				}
			}
			return nil
		}
		name := d.Name()
		if name == ".go-arch-lint" || name == "vendor" || strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, p)
		relSlash := filepath.ToSlash(rel)
		if name == "testdata" {
			// testdata is outside the architecture but still scanned by
			// check: exclude it explicitly, or a fresh scaffold is red.
			excludes = append(excludes, relSlash+"/**")
			return filepath.SkipDir
		}
		if hasModuleFile(p) {
			// a nested module is its own unit: its packages are not
			// importable from this module, so it is not declared — but
			// the scanner still walks the tree, so exclude it too.
			excludes = append(excludes, relSlash+"/**")
			return filepath.SkipDir
		}
		entries, readErr := os.ReadDir(p)
		if readErr != nil {
			return nil //nolint:nilerr // unreadable directories are skipped
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				dirs = append(dirs, relSlash)
				break
			}
		}
		return nil
	})
	sort.Strings(dirs)
	sort.Strings(excludes)
	return dirs, excludes
}

// moduleFileName is the module boundary marker: a directory holding it is
// its own Go module and not part of this module's package graph.
const moduleFileName = "go.mod"

// hasModuleFile reports whether dir contains its own go.mod.
func hasModuleFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() == moduleFileName {
			return true
		}
	}
	return false
}

// v2SpecFromDirs renders the scaffolded arch.go body declaring every
// discovered Go directory as a Path component. A module without any Go
// subdirectory falls back to the module root alone.
func v2SpecFromDirs(dirs, excludes []string) string {
	var b strings.Builder
	b.WriteString("var build = Spec(func() {\n")
	b.WriteString("\t// Every directory with Go code is declared: the language\n")
	b.WriteString("\t// fails on undeclared directories. Add Use rules as your\n")
	b.WriteString("\t// architecture takes shape:\n")
	b.WriteString("\t//\n")
	b.WriteString("\t//     Path(\"internal/core\", func() { Use(domain) })\n")
	if len(dirs) == 0 {
		b.WriteString("\tPath(\".\")\n")
	} else {
		for _, d := range dirs {
			fmt.Fprintf(&b, "\tPath(%q)\n", d)
		}
	}
	if len(excludes) > 0 {
		b.WriteString("\t// Outside the architecture, but inside the tree:\n")
		b.WriteString("\tExclude(")
		for i, e := range excludes {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", e)
		}
		b.WriteString(")\n")
	}
	b.WriteString("})\n")
	return b.String()
}

// scaffoldPrefix is everything of scaffoldArchGo before the spec body —
// the fixed header the generated declaration list is appended to.
func scaffoldPrefix() string {
	cut := strings.Index(scaffoldArchGo, "var build =")
	if cut < 0 {
		// unreachable while scaffoldArchGo is a compile-time constant
		// with the marker; kept defensive for future template edits
		return scaffoldArchGo
	}
	return scaffoldArchGo[:cut]
}

func cmdInit(args []string) int {
	for _, a := range args {
		if a == flagHelp || a == "-h" {
			printInitUsage()
			return 0
		}
	}

	projectPath, err := parseInitArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Default scaffold: declare the project's real directory tree so
	// the fresh spec passes the declare-everything rule day one.
	prefix := scaffoldPrefix()
	dirs, excludes := scanGoDirs(projectPath)
	body := prefix + v2SpecFromDirs(dirs, excludes) //nolint:gosec // project path is a user-specified CLI argument, same trust level as the rest of init
	return writeScaffold(projectPath, body, scaffoldMainGo, "")
}

// writeScaffold creates the .go-arch-lint directory with go.mod, arch.go and
// main.go. archGo is the spec body; label is
// appended to the final "scaffold created" line, may be empty.
func writeScaffold(projectPath, archGo, mainGo, label string) int {
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
		"main.go": mainGo,
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
	fmt.Printf("  2. Run 'cd %s && go mod tidy' to resolve the github.com/vsfedorenko/go-arch-lint/v3 dependency\n", archDir)
	fmt.Printf("  3. Run 'go-arch-lint check' to lint your project\n")

	return 0
}
