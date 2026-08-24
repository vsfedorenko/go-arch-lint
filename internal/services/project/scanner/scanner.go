package scanner

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
	astUtil "github.com/vsfedorenko/go-arch-lint/v3/internal/services/common/ast"
)

type (
	Scanner struct {
		stdPackages map[string]struct{}
	}

	resolveContext struct {
		projectDirectory    string
		moduleName          string
		workspaceModules    []string
		excludePaths        []models.ResolvedPath
		excludeFileMatchers []*regexp.Regexp

		tokenSet *token.FileSet
		results  []models.ProjectFile
	}
)

func NewScanner() *Scanner {
	scanner := &Scanner{
		stdPackages: make(map[string]struct{}, 255),
	}

	stdPackages, err := packages.Load(nil, "std")
	if err != nil {
		panic(fmt.Errorf("failed load std packages"))
	}

	for _, stdPackage := range stdPackages {
		scanner.stdPackages[stdPackage.ID] = struct{}{}
	}

	return scanner
}

func (r *Scanner) Scan(
	ctx context.Context,
	projectDirectory string,
	moduleName string,
	excludePaths []models.ResolvedPath,
	excludeFileMatchers []*regexp.Regexp,
) ([]models.ProjectFile, error) {
	return r.ScanInWorkspace(ctx, nil, projectDirectory, moduleName, excludePaths, excludeFileMatchers)
}

// ScanInWorkspace is Scan with go.work sibling modules: imports of those
// modules classify as project imports instead of vendor.
func (r *Scanner) ScanInWorkspace(
	_ context.Context,
	workspaceModules []domain.WorkspaceModule,
	projectDirectory string,
	moduleName string,
	excludePaths []models.ResolvedPath,
	excludeFileMatchers []*regexp.Regexp,
) ([]models.ProjectFile, error) {
	modulePaths := make([]string, 0, len(workspaceModules))
	for _, module := range workspaceModules {
		modulePaths = append(modulePaths, module.Path)
	}

	rctx := resolveContext{
		projectDirectory:    projectDirectory,
		moduleName:          moduleName,
		workspaceModules:    modulePaths,
		excludePaths:        excludePaths,
		excludeFileMatchers: excludeFileMatchers,

		tokenSet: token.NewFileSet(),
		results:  []models.ProjectFile{},
	}

	err := filepath.Walk(rctx.projectDirectory, func(path string, info os.FileInfo, err error) error {
		return r.resolveFile(&rctx, path, info, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk project tree: %w", err)
	}

	return rctx.results, nil
}

func (r *Scanner) resolveFile(ctx *resolveContext, path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Skip descending into excluded directories entirely.
		if r.isExcludedDir(ctx, path) {
			return filepath.SkipDir
		}
		return nil
	}

	if !r.inScope(ctx, path) {
		return nil
	}

	return r.parse(ctx, path)
}

func (r *Scanner) isExcludedDir(ctx *resolveContext, path string) bool {
	for _, excludePath := range ctx.excludePaths {
		if path == excludePath.AbsPath ||
			strings.HasPrefix(path, excludePath.AbsPath+string(os.PathSeparator)) {
			return true
		}
	}

	return false
}

func (r *Scanner) inScope(ctx *resolveContext, path string) bool {
	if filepath.Ext(path) != ".go" {
		return false
	}

	for _, excludePath := range ctx.excludePaths {
		if strings.HasPrefix(path, excludePath.AbsPath) {
			return false
		}
	}

	for _, matcher := range ctx.excludeFileMatchers {
		if matcher.Match([]byte(path)) {
			return false
		}
	}

	return true
}

func (r *Scanner) parse(ctx *resolveContext, path string) error {
	fileAst, err := parser.ParseFile(ctx.tokenSet, path, nil, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("failed to parse go source code at '%s': %w", path, err)
	}

	ctx.results = append(ctx.results, models.ProjectFile{
		Path:        path,
		PackageName: fileAst.Name.Name,
		Imports:     r.extractImports(ctx, fileAst),
	})

	return nil
}

func (r *Scanner) extractImports(ctx *resolveContext, fileAst *ast.File) []models.ResolvedImport {
	imports := make([]models.ResolvedImport, 0)

	for _, goImport := range fileAst.Imports {
		importPath := strings.Trim(goImport.Path.Value, "\"")
		imports = append(imports, models.ResolvedImport{
			Name:       importPath,
			ImportType: models.GetImportTypeInWorkspace(importPath, ctx.moduleName, ctx.workspaceModules, r.stdPackages),
			Reference:  astUtil.PositionFromToken(ctx.tokenSet.Position(goImport.Pos())),
		})
	}

	return imports
}
