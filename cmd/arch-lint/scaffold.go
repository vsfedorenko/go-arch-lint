package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const scaffoldGoMod = `module arch-lint-local

go 1.25
`

const scaffoldMainGo = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")

		Allow(func() {
			DepOnAnyVendor(false)
		})

		ExcludeFiles(` + "`^.*_test\\.go$`" + `)

		// Define your components:
		// Component("main", "app")
		// Component("services", "services/**")

		// Define dependency rules:
		// Deps("main", func() {
		//     MayDependOn("services")
		// })
	})
	archlint.MustRun(spec)
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
	fmt.Printf("  1. Edit %s/main.go to describe your architecture\n", archDir)
	fmt.Printf("  2. Run 'cd %s && go mod tidy' to resolve the github.com/vsfedorenko/go-arch-lint dependency\n", archDir)
	fmt.Printf("  3. Run 'go-arch-lint check' to lint your project\n")

	return 0
}
