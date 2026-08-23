package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
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
// args. Two fail-fast contracts, both pinned by tests:
//   - a flag present without its value (last token or followed by another
//     flag) is an error — silently scaffolding at the wrong path writes the
//     wrong starting point;
//   - any OTHER token is an error naming the token: init takes no positional
//     arguments, and an unknown flag (e.g. the removed `--recipe`) silently
//     scaffolding the default spec is exactly the lenient-flag bug class
//     (#48, #114).
func parseInitArgs(args []string) (projectPath string, err error) {
	projectPath = "."
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == flagProjectPath || a == "-p":
			if i+1 >= len(args) || isFlagLike(args[i+1]) {
				return "", fmt.Errorf("%s requires a value (the project directory)", a)
			}
			projectPath = args[i+1]
			i++
		case strings.HasPrefix(a, "--project-path="):
			projectPath = strings.TrimPrefix(a, "--project-path=")
		default:
			return "", fmt.Errorf("unknown flag or argument: %s\ninit takes no positional arguments; the only flag is --project-path/-p", a)
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
// discovered Go directory as a Path component, with Use rules mirroring
// the imports that already exist (dependencies declared first, so the
// spec compiles). A module without any Go subdirectory falls back to the
// module root alone.
func v2SpecFromDirs(dirs, excludes []string, edges map[string][]string) string {
	names := specVarNames(dirs)
	// A path needs a variable only when it is REFERENCED by a later Use
	// rule (a Use target). Sources are consumed by their own statement.
	needsVar := map[string]bool{}
	for _, targets := range edges {
		for _, t := range targets {
			needsVar[t] = true
		}
	}
	var b strings.Builder
	b.WriteString("var build = Spec(func() {\n")
	b.WriteString("	// Every directory with Go code is declared, and every import\n")
	b.WriteString("	// that exists today is allowed: the scaffold mirrors the code\n")
	b.WriteString("	// as-is. Tighten the Use rules as your architecture takes shape:\n")
	b.WriteString("	//\n")
	b.WriteString("	//     domain := Path(\"internal/domain\")\n")
	b.WriteString("	//     Path(\"internal/core\", func() { Use(domain) })\n")
	if len(dirs) == 0 {
		// A module with no Go files yet: the root alone keeps the spec
		// valid (at least one component must be defined).
		b.WriteString("	Path(\".\")\n")
	}
	for _, d := range specDeclOrder(dirs, edges) {
		targets := edges[d]
		if len(targets) > 0 {
			uses := make([]string, 0, len(targets))
			for _, t := range targets {
				uses = append(uses, names[t])
			}
			decl := fmt.Sprintf("Path(%q, func() { Use(%s) })", d, strings.Join(uses, ", "))
			if needsVar[d] {
				fmt.Fprintf(&b, "	%s := %s\n", names[d], decl)
			} else {
				fmt.Fprintf(&b, "	%s\n", decl)
			}
			continue
		}
		if needsVar[d] {
			fmt.Fprintf(&b, "	%s := Path(%q)\n", names[d], d)
		} else {
			fmt.Fprintf(&b, "	Path(%q)\n", d)
		}
	}
	if len(excludes) > 0 {
		b.WriteString("	// Outside the architecture, but inside the tree:\n")
		b.WriteString("	Exclude(")
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

// specVarName derives the Go identifier for a declared path: the module
// root becomes "root", every other directory its last element (sanitized
// to letters/digits, lowercased, with a "p" fallback for empty names so
// the identifier is never a Go keyword clash handled below).
func specVarName(dir string) string {
	if dir == "." {
		return "root"
	}
	base := path.Base(dir)
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 'a' - 'A')
		default:
			// separators and symbols fold away
		}
	}
	name := b.String()
	if name == "" {
		return "p"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "p" + name
	}
	return name
}

// goKeywords are reserved words a scaffolded variable name must not use.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// specVarNames assigns a unique Go identifier to every declared path:
// the module root becomes "root", other directories take their last
// element (lowercased, symbols folded away). Collisions get a numeric
// suffix; Go keywords and the reserved "build" get a trailing underscore.
func specVarNames(dirs []string) map[string]string {
	names := make(map[string]string, len(dirs))
	taken := map[string]bool{}
	for _, d := range dirs {
		base := specVarName(d)
		if goKeywords[base] || base == "build" {
			base += "_"
		}
		name := base
		for i := 2; taken[name]; i++ {
			name = fmt.Sprintf("%s%d", base, i)
		}
		taken[name] = true
		names[d] = name
	}
	return names
}

// specDeclOrder orders declarations dependencies-first (Kahn's algorithm
// over the Use edges) so every referenced variable is declared before its
// first Use. Non-test Go imports cannot cycle, but the sort is defensive:
// any leftover (impossible cycle) appends in sorted order rather than
// looping forever.
func specDeclOrder(dirs []string, edges map[string][]string) []string {
	sorted := append([]string(nil), dirs...)
	sort.Strings(sorted)
	deps := make(map[string][]string, len(edges))
	indeg := make(map[string]int, len(sorted))
	for _, d := range sorted {
		indeg[d] = 0
	}
	for from, targets := range edges {
		for _, t := range targets {
			if t == from {
				continue
			}
			deps[t] = append(deps[t], from)
			indeg[from]++
		}
	}
	var queue []string
	for _, d := range sorted {
		if indeg[d] == 0 {
			queue = append(queue, d)
		}
	}
	var order []string
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		order = append(order, d)
		for _, dependent := range deps[d] {
			indeg[dependent]--
			if indeg[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	if len(order) == len(sorted) {
		return order
	}
	return sorted // unreachable with real Go imports; defensive fallback
}

// scanImports maps every declared directory to the declared directories
// its non-test Go files import. Import paths are matched against the
// module path from go.mod: module "probe" makes "probe/internal/hello"
// resolve to the directory "internal/hello". Test files are skipped —
// the checker does not flag test imports, and test-only edges may cycle.
func scanImports(root string, dirs []string) map[string][]string {
	edges := make(map[string][]string, len(dirs))
	if moduleName(root) == "" {
		return edges
	}
	byImport := make(map[string]string, len(dirs)+1)
	for _, d := range dirs {
		if d == "." {
			byImport[moduleName(root)] = "."
			continue
		}
		byImport[moduleName(root)+"/"+d] = d
	}
	fset := token.NewFileSet()
	for _, d := range dirs {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(d)))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			file := filepath.Join(root, filepath.FromSlash(d), e.Name())
			astFile, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
			if err != nil {
				continue
			}
			for _, imp := range astFile.Imports {
				target, ok := byImport[strings.Trim(imp.Path.Value, `"`)]
				if !ok || target == d || seen[target] {
					continue
				}
				seen[target] = true
				edges[d] = append(edges[d], target)
			}
		}
		sort.Strings(edges[d])
	}
	return edges
}

// moduleName reads the module path from the go.mod at root. An empty
// string (no go.mod, unreadable, unparsable) disables import scanning.
func moduleName(root string) string {
	data, err := os.ReadFile(filepath.Join(root, moduleFileName)) //nolint:gosec // root is the init --project-path argument, same trust level as every other init read
	if err != nil {
		return ""
	}
	f, err := modfile.Parse(moduleFileName, data, nil)
	if err != nil {
		return ""
	}
	if f.Module == nil {
		return ""
	}
	return f.Module.Mod.Path
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
	// the fresh spec passes the declare-everything rule day one, with
	// Use rules mirroring the imports that already exist.
	prefix := scaffoldPrefix()
	dirs, excludes := scanGoDirs(projectPath)
	edges := scanImports(projectPath, dirs)
	body := prefix + v2SpecFromDirs(dirs, excludes, edges) //nolint:gosec // project path is a user-specified CLI argument, same trust level as the rest of init
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
