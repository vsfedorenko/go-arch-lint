package info

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

type Assembler struct{}

func NewAssembler() *Assembler {
	return &Assembler{}
}

func (a *Assembler) ProjectInfo(rootDirectory string, archFilePath string) (domain.Project, error) {
	projectPath, err := filepath.Abs(rootDirectory)
	if err != nil {
		return domain.Project{}, fmt.Errorf("failed to resolve abs path '%s'", rootDirectory)
	}

	// check arch file
	goArchFilePath, err := resolveArchPath(projectPath, archFilePath)
	if err != nil {
		return domain.Project{}, err
	}

	// check go.mod
	goModFilePath := filepath.Clean(fmt.Sprintf("%s/%s", projectPath, models.DefaultGoModFileName))
	_, err = os.Stat(goModFilePath)
	if os.IsNotExist(err) {
		return domain.Project{}, fmt.Errorf("not found project '%s' in '%s'%s",
			models.DefaultGoModFileName,
			goModFilePath,
			workspaceHint(projectPath),
		)
	}

	// parse go.mod
	moduleName, err := checkCmdExtractModuleName(goModFilePath)
	if err != nil {
		return domain.Project{}, fmt.Errorf("failed get module name: %w", err)
	}

	workspaceModules, err := collectWorkspaceModules(projectPath)
	if err != nil {
		return domain.Project{}, err
	}

	return domain.Project{
		Directory:        projectPath,
		GoArchFilePath:   goArchFilePath,
		GoModFilePath:    goModFilePath,
		ModuleName:       moduleName,
		WorkspaceModules: workspaceModules,
	}, nil
}

// collectWorkspaceModules parses the project's go.work (if any) and returns
// its `use` entries with the module path each member declares in its own
// go.mod. Entries whose go.mod is missing or unreadable are skipped — the
// arch check must not fail because a workspace lists a module outside the
// linted tree. The root module itself is not listed (see domain.Project).
func collectWorkspaceModules(projectPath string) ([]domain.WorkspaceModule, error) {
	goWorkPath := filepath.Join(projectPath, models.DefaultGoWorkFileName)
	body, err := os.ReadFile(goWorkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read '%s': %w", goWorkPath, err)
	}

	work, err := modfile.ParseWork(goWorkPath, body, nil)
	if err != nil {
		// A malformed go.work must not break projects that do not rely on
		// workspace semantics: the `go` tooling itself refuses such files,
		// but the arch lint keeps working with root-module-only knowledge.
		return nil, nil //nolint:nilerr // deliberate degradation to non-workspace mode
	}

	modules := make([]domain.WorkspaceModule, 0, len(work.Use))
	for _, use := range work.Use {
		// `use <dir>` carries the member directory; the module path comes
		// from the member's own go.mod.
		dir := filepath.Clean(filepath.Join(projectPath, use.Path))
		if dir == filepath.Clean(projectPath) {
			continue // root module — already covered by ModuleName
		}

		modulePath, err := checkCmdExtractModuleName(filepath.Join(dir, models.DefaultGoModFileName))
		if err != nil {
			continue
		}

		modules = append(modules, domain.WorkspaceModule{Dir: dir, Path: modulePath})
	}

	return modules, nil
}

// workspaceHint explains the missing-go.mod failure when the directory is a
// Go workspace root: go-arch-lint resolves packages relative to a root
// go.mod, and a workspace of sibling modules without one has no module to
// lint. The hint names the two working layouts instead of leaving a bare
// "not found" error.
func workspaceHint(projectPath string) string {
	goWorkPath := filepath.Join(projectPath, models.DefaultGoWorkFileName)
	if _, err := os.Stat(goWorkPath); err != nil {
		return ""
	}
	return fmt.Sprintf(
		" (found %s without a root %s: run go-arch-lint from a directory with its own %s; "+
			"a workspace of sibling modules with no root module is not supported yet)",
		models.DefaultGoWorkFileName,
		models.DefaultGoModFileName,
		models.DefaultGoModFileName,
	)
}

func checkCmdExtractModuleName(goModPath string) (string, error) {
	goModFile, err := checkCmdParseGoModFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("can`t parse gomod: %w", err)
	}

	// modfile.ParseLax returns (nil, err) on syntax errors and may return
	// a file without a Module block for files that parse but declare no
	// module — dereferencing either panics.
	if goModFile == nil || goModFile.Module == nil {
		return "", fmt.Errorf("%s should contain valid go.mod syntax and 'module' directive", models.DefaultGoModFileName)
	}

	moduleName := goModFile.Module.Mod.Path
	if moduleName == "" {
		return "", fmt.Errorf("%s should contain module name in 'module'", models.DefaultGoModFileName)
	}

	return moduleName, nil
}

func checkCmdParseGoModFile(path string) (*modfile.File, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", path, err)
	}

	mod, err := modfile.ParseLax(path, file, nil)
	if err != nil {
		return nil, fmt.Errorf("modfile parseLax failed '%s': %w", path, err)
	}

	return mod, nil
}

func resolveArchPath(projectPath, archFilePath string) (string, error) {
	if archFileURL, err := url.Parse(archFilePath); err == nil && archFileURL.Scheme != "" {
		return checkArchFileURL(archFilePath)
	}

	if filepath.IsAbs(archFilePath) {
		return checkArchFile(archFilePath)
	}

	return checkArchFile(filepath.Join(projectPath, archFilePath))
}

func checkArchFile(archFilePath string) (string, error) {
	// GoDecoder reads from in-memory SpecBuilder; arch file need not exist.
	return archFilePath, nil
}

func checkArchFileURL(archFileURL string) (string, error) {
	return "", errors.New("URL arch-file loading is not supported in v3 Go DSL mode; use a local .go-arch-lint/arch.go")
}
