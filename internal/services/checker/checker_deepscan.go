package checker

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/checker/deepscan"
)

type DeepScan struct {
	sourceCodeRenderer sourceCodeRenderer

	scanner           *deepscan.Searcher
	spec              arch.Spec
	result            models.CheckResult
	fileComponents    map[string]string
	packageComponents map[string]string

	// checkMux serialises concurrent Check() calls on the shared state
	// below (spec, result, component maps) — the checker is a long-lived
	// singleton in the container.
	checkMux sync.Mutex

	// resultMux protects result.DeepscanWarnings from concurrent appends
	// when multiple workers scan components in parallel.
	resultMux sync.Mutex
}

func NewDeepScan(sourceCodeRenderer sourceCodeRenderer) *DeepScan {
	return &DeepScan{
		sourceCodeRenderer: sourceCodeRenderer,
		scanner:            deepscan.NewSearcher(),
	}
}

// workersCount returns the number of concurrent workers used to scan
// project components in parallel.
//
// Previously this returned a hard-coded 1 with a TODO noting the scan
// algorithm serialized on a mutex. The shared deepscan.Searcher is still
// internally serialized by its own mutex, but the per-component work
// (AST loading, usage matching, result building) is independent and can
// overlap across workers. To make that safe, result appends are now
// protected by resultMux.
//
// We cap at 8 to avoid excessive goroutine/cache pressure on machines
// with many cores while still utilizing available CPU.
func (c *DeepScan) workersCount() int {
	const maxWorkers = 8
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > maxWorkers {
		n = maxWorkers
	}
	return n
}

func (c *DeepScan) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	maxWorkers := c.workersCount()

	c.checkMux.Lock()
	defer c.checkMux.Unlock()

	// -- prepare shared objects
	c.spec = spec
	c.result = models.CheckResult{}

	// -- prepare mapping file -> component (resolved once by the composite)
	c.fileComponents = map[string]string{}
	c.packageComponents = map[string]string{}

	for _, hold := range projectFiles {
		if hold.ComponentID == nil {
			continue
		}

		// cache file -> component ref
		c.fileComponents[hold.File.Path] = *hold.ComponentID

		// cache package -> component ref
		packagePath := filepath.Dir(hold.File.Path)
		c.packageComponents[packagePath] = *hold.ComponentID
	}

	// -- scan project
	pool := make(chan struct{}, maxWorkers)
	var wg errgroup.Group

	for _, component := range spec.Components {
		pool <- struct{}{}
		wg.Go(func() error {
			defer func() {
				<-pool
			}()

			if !component.DeepScan.Value {
				return nil
			}

			err := c.checkComponent(ctx, component)
			if err != nil {
				return fmt.Errorf("component '%s' check failed: %w",
					component.Name.Value,
					err,
				)
			}

			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return models.CheckResult{}, err
	}

	return c.result, nil
}

func (c *DeepScan) checkComponent(ctx context.Context, cmp arch.Component) error {
	for _, packagePath := range cmp.ResolvedPaths {
		absPath := packagePath.Value.AbsPath
		matchedCmp, ok := c.packageComponents[absPath]
		if !ok {
			// component in excludes list
			continue
		}

		if matchedCmp != cmp.Name.Value {
			// this can be in cased of wildcard match, example:
			// cmp1: in: internal/code/common
			// cmp2: in: internal/code/**
			// and when cmp.Name == cmp2, this cmp2 still have "code/common" in resolvedPath's from cmp1
			// here we can skip this, because deps of cmp1 will be checked later.
			continue
		}

		err := c.scanPackage(ctx, &cmp, absPath)
		if err != nil {
			return fmt.Errorf("failed scan '%s': %w", absPath, err)
		}
	}

	return nil
}

func (c *DeepScan) scanPackage(ctx context.Context, cmp *arch.Component, absPackagePath string) error {
	usages, err := c.findUsages(ctx, absPackagePath)
	if err != nil {
		return fmt.Errorf("find usages failed: %w", err)
	}

	if len(usages) == 0 {
		return nil
	}

	for _, usage := range usages {
		err := c.checkUsage(ctx, cmp, &usage)
		if err != nil {
			return fmt.Errorf("failed check usage '%s' in '%s': %w",
				usage.Name,
				usage.Definition.Place,
				err,
			)
		}
	}

	return nil
}

func (c *DeepScan) checkUsage(ctx context.Context, cmp *arch.Component, usage *deepscan.InjectionMethod) error {
	for _, gate := range usage.Gates {
		if len(gate.Implementations) == 0 {
			continue
		}

		err := c.checkGate(ctx, cmp, &gate)
		if err != nil {
			return fmt.Errorf("failed check gate '%s': %w",
				gate.ArgumentDefinition.Place,
				err,
			)
		}
	}

	return nil
}

func (c *DeepScan) checkGate(_ context.Context, cmp *arch.Component, gate *deepscan.Gate) error {
	for _, implementation := range gate.Implementations {
		err := c.checkImplementation(cmp, gate, &implementation)
		if err != nil {
			return fmt.Errorf("failed check implementation '%v': %w",
				implementation.Injector.ParamDefinition,
				err,
			)
		}
	}

	return nil
}

//nolint:unparam // error result is part of the composite check contract; implementations may fail
func (c *DeepScan) checkImplementation(
	cmp *arch.Component,
	gate *deepscan.Gate,
	imp *deepscan.Implementation,
) error {
	injectedImport := imp.Target.Definition.Import

	// allow explicitly allowed project imports
	for _, allowedImport := range cmp.AllowedProjectImports {
		if allowedImport.Value.ImportPath == injectedImport.Name {
			return nil
		}
	}

	// allow by special flags per import type
	if cmp.SpecialFlags.AllowAllProjectDeps.Value && injectedImport.ImportType == models.ImportTypeProject {
		return nil
	}
	if cmp.SpecialFlags.AllowAllVendorDeps.Value && injectedImport.ImportType == models.ImportTypeVendor {
		return nil
	}

	targetPath := imp.Target.Definition.Place.File
	targetComponentID, targetDefined := c.fileComponents[targetPath]

	gatePath := gate.MethodDefinition.Place.File
	gateComponentID, gateDefined := c.fileComponents[gatePath]

	if !targetDefined || !gateDefined {
		// target component is vendor or std file, not described in mapping
		// we can check vendor libs too, but this requires another scan process

		// example of skipping target:
		// - $GOROOT/src/context/context.go (stdlib)
		// - /home/neo/go/src/example.com/ns/awesome/vendor/libs.example.com/good/producer/client.go (vendor)
		return nil
	}

	warn := models.CheckArchWarningDeepscan{
		Gate: models.DeepscanWarningGate{
			ComponentName: gateComponentID,
			MethodName:    gate.MethodName,
			RelativePath:  c.definitionToRelPath(gate.ArgumentDefinition.Place),
			Definition:    gate.ArgumentDefinition.Place,
		},
		Dependency: models.DeepscanWarningDependency{
			ComponentName: targetComponentID,
			Name: fmt.Sprintf("%s.%s",
				imp.Target.Definition.Pkg,
				imp.Target.StructName,
			),
			InjectionAST:  imp.Injector.CodeName,
			Injection:     imp.Injector.ParamDefinition.Place,
			InjectionPath: c.definitionToRelPath(imp.Injector.ParamDefinition.Place),
			SourceCodePreview: c.renderCode(
				imp.Injector.ParamDefinition.Place,
				imp.Injector.MethodDefinition.Place,
				imp.Injector.ParamDefinition.Place,
			),
		},
		Target: models.DeepscanWarningTarget{
			Definition:   imp.Target.Definition.Place,
			RelativePath: c.definitionToRelPath(imp.Target.Definition.Place),
		},
	}

	c.resultMux.Lock()
	c.result.DeepscanWarnings = append(c.result.DeepscanWarnings, warn)
	c.resultMux.Unlock()
	return nil
}

func (c *DeepScan) renderCode(pointer, from, to domain.Reference) []byte {
	return c.sourceCodeRenderer.SourceCode(
		domain.NewReferenceRange(pointer.File, from.Line, pointer.Line, to.Line),
		false,
		false,
	)
}

func (c *DeepScan) definitionToRelPath(source domain.Reference) string {
	relativePath := strings.TrimPrefix(source.File, c.spec.RootDirectory.Value)
	return fmt.Sprintf("%s:%d", relativePath, source.Line)
}

func (c *DeepScan) findUsages(_ context.Context, absPackagePath string) ([]deepscan.InjectionMethod, error) {
	scanDirectory := path.Clean(fmt.Sprintf("%s/%s",
		c.spec.RootDirectory.Value,
		c.spec.WorkingDirectory.Value,
	))
	excludeDirectories := c.refPathToList(c.spec.Exclude)
	excludeMatchers := c.refRegexpToList(c.spec.ExcludeFilesMatcher)

	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(absPackagePath),
		deepscan.WithAnalyseScope(scanDirectory),
		deepscan.WithExcludedPath(excludeDirectories),
		deepscan.WithExcludedFileMatchers(excludeMatchers),
	)
	if err != nil {
		return nil, fmt.Errorf("failed prepare scan criteria: %w", err)
	}

	usages, err := c.scanner.Usages(criteria)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return usages, nil
}

func (c *DeepScan) refPathToList(list []domain.Referable[models.ResolvedPath]) []string {
	result := make([]string, 0)

	for _, refPath := range list {
		result = append(result, refPath.Value.AbsPath)
	}

	return result
}

func (c *DeepScan) refRegexpToList(list []domain.Referable[*regexp.Regexp]) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0)

	for _, refPath := range list {
		result = append(result, refPath.Value)
	}

	return result
}
