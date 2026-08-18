package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const scaffoldGoMod = `module arch-lint-local

go 1.25
`

// scaffoldArchGo holds the user-editable spec. It lives in its own file so
// the runner (main.go) can be regenerated or upgraded without touching the
// architecture description.
const scaffoldArchGo = `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(` + "`^.*_test\\\\.go$`" + `)

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
const scaffoldMainGo = `package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint"
)

func main() {
	archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)
}
`

func cmdInit(args []string) int {
	projectPath := "."
	for i, a := range args {
		if (a == "--project-path" || a == "-p") && i+1 < len(args) {
			projectPath = args[i+1]
			break
		}
	}

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
		"arch.go": scaffoldArchGo,
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

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit %s/arch.go to describe your architecture\n", archDir)
	fmt.Printf("  2. Run 'cd %s && go mod tidy' to resolve the github.com/vsfedorenko/go-arch-lint dependency\n", archDir)
	fmt.Printf("  3. Run 'go-arch-lint check' to lint your project\n")

	return 0
}
