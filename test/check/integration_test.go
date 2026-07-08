package check_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func testProjectDir(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(root, "test", "check", "project")
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func scaffoldArch(t *testing.T, repoRoot, mainGo string) string {
	t.Helper()
	dir := t.TempDir()

	goMod := fmt.Sprintf(`module arch-lint-local

go 1.25

require github.com/vsfedorenko/go-arch-lint v0.0.0

replace github.com/vsfedorenko/go-arch-lint => %s
`, repoRoot)

	files := map[string]string{
		"go.mod":  goMod,
		"main.go": mainGo,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	return dir
}

func runArchLint(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOFLAGS=-mod=mod",
		"GOSUMDB=off",
		"GOPROXY=off",
	)

	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run go run: %v\nstderr: %s", err, errb.String())
		}
	}
	return out.String(), errb.String(), exitCode
}

const archOK = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c/**")
		Component("d", "internal/d/**")
		Component("e", "internal/e/**")
		Component("nc", "internal/not_covered")
		Component("common", "internal/common/**")
		CommonComponents("common", "a", "c", "d", "e")
		Deps("allowb", func() { MayDependOn("b") })
		Deps("e", func() { AnyVendorDeps(true) })
	})
	archlint.MustRun(spec)
}
`

const archWarnings = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal/.")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c")
		Component("e", "internal/e")
		Component("common", "internal/common/**")
		Component("models", "internal/d/models/*/model")
		CommonComponents("common")
		Deps("e", func() {
			MayDependOn("models")
			AnyVendorDeps(true)
		})
		Deps("allowb", func() { MayDependOn("b") })
	})
	archlint.MustRun(spec)
}
`

const archInvalidSpec = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Component("main", "internal")
		Component("a", "internal/a")
		Component("not_exist", "internal/not_exist")
		CommonComponents("models")
		Deps("main", func() {
			MayDependOn("not_exist_too_rnd_order")
		})
	})
	archlint.MustRun(spec)
}
`

func TestCheckCommands(t *testing.T) {
	project := testProjectDir(t)

	tests := []struct {
		name       string
		archGo     string
		args       []string
		wantExit   int
		wantOutput string
	}{
		{
			name:       "ok",
			archGo:     archOK,
			args:       []string{"check", "--project-path", project, "--output-color=false"},
			wantExit:   0,
			wantOutput: "OK - No warnings found",
		},
		{
			name:       "warnings",
			archGo:     archWarnings,
			args:       []string{"check", "--project-path", project, "--output-color=false"},
			wantExit:   1,
			wantOutput: "Component c shouldn't depend on",
		},
		{
			name:       "invalid_spec",
			archGo:     archInvalidSpec,
			args:       []string{"check", "--project-path", project, "--output-color=false"},
			wantExit:   1,
			wantOutput: "not found directories",
		},
		{
			name:       "help",
			archGo:     archOK,
			args:       []string{"check", "--help"},
			wantExit:   0,
			wantOutput: "compare project *.go files with arch defined in spec file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := repoRoot(t)
			dir := scaffoldArch(t, root, tt.archGo)

			stdout, stderr, exitCode := runArchLint(t, dir, tt.args...)

			combined := stdout + stderr
			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", exitCode, tt.wantExit, stdout, stderr)
			}
			if !strings.Contains(combined, tt.wantOutput) {
				t.Errorf("output does not contain %q\nstdout:\n%s\nstderr:\n%s", tt.wantOutput, stdout, stderr)
			}
		})
	}
}
