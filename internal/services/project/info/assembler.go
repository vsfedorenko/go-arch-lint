package info

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
)

type Assembler struct{}

func NewAssembler() *Assembler {
	return &Assembler{}
}

func (a *Assembler) ProjectInfo(rootDirectory string, archFilePath string) (common.Project, error) {
	projectPath, err := filepath.Abs(rootDirectory)
	if err != nil {
		return common.Project{}, fmt.Errorf("failed to resolve abs path '%s'", rootDirectory)
	}

	// check arch file
	goArchFilePath, err := resolveArchPath(projectPath, archFilePath)
	if err != nil {
		return common.Project{}, err
	}

	// check go.mod
	goModFilePath := filepath.Clean(fmt.Sprintf("%s/%s", projectPath, models.DefaultGoModFileName))
	_, err = os.Stat(goModFilePath)
	if os.IsNotExist(err) {
		return common.Project{}, fmt.Errorf("not found project '%s' in '%s'",
			models.DefaultGoModFileName,
			goModFilePath,
		)
	}

	// parse go.mod
	moduleName, err := checkCmdExtractModuleName(goModFilePath)
	if err != nil {
		return common.Project{}, fmt.Errorf("failed get module name: %w", err)
	}

	return common.Project{
		Directory:      projectPath,
		GoArchFilePath: goArchFilePath,
		GoModFilePath:  goModFilePath,
		ModuleName:     moduleName,
	}, nil
}

func checkCmdExtractModuleName(goModPath string) (string, error) {
	goModFile, err := checkCmdParseGoModFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("can`t parse gomod: %w", err)
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
	return "", errors.New("URL arch-file loading is not supported in v2.0 Go DSL mode; use a local .go-arch-lint/arch.go")
}
