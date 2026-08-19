package container

import (
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/checker"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/common/path"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/project/holder"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/project/info"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/project/resolver"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/project/scanner"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/render/code"
	specassembler "github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec/assembler"
	specvalidator "github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec/validator"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/suppress"
)

func (c *Container) provideSpecAssembler() *specassembler.Assembler {
	return specassembler.NewAssembler(
		c.provideGoSpecProvider(),
		c.provideSpecValidator(),
		c.providePathResolver(),
	)
}

func (c *Container) provideSpecValidator() *specvalidator.Validator {
	return specvalidator.NewValidator(
		c.providePathResolver(),
	)
}

func (c *Container) provideGoSpecProvider() SpecDecoder {
	return c.externalDecoder
}

func (c *Container) providePathResolver() *path.Resolver {
	return path.NewResolver()
}

func (c *Container) provideReferenceRender() *code.Render {
	return code.NewRender(
		c.provideColorPrinter(),
	)
}

func (c *Container) provideSpecChecker() *checker.CompositeChecker {
	return checker.NewCompositeChecker(
		c.provideProjectFilesResolver(),
		c.provideSpecImportsChecker(),
		c.provideSpecCyclesChecker(),
		c.provideSpecTiersChecker(),
		c.provideSpecNamingChecker(),
		c.provideSpecInterfacePlacementChecker(),
		c.provideSpecVisibilityChecker(),
		c.provideSpecDeepScanChecker(),
	).WithSuppressIndex(func(projectFiles []models.FileHold) (checker.SuppressIndex, error) {
		paths := make([]string, 0, len(projectFiles))
		for _, hold := range projectFiles {
			paths = append(paths, hold.File.Path)
		}

		return suppress.NewIndexFromFiles(paths)
	})
}

func (c *Container) provideSpecInterfacePlacementChecker() *checker.InterfacePlacement {
	return checker.NewInterfacePlacement()
}

func (c *Container) provideSpecNamingChecker() *checker.Naming {
	return checker.NewNaming()
}

func (c *Container) provideSpecTiersChecker() *checker.TierRules {
	return checker.NewTierRules()
}

func (c *Container) provideSpecCyclesChecker() *checker.Cycles {
	return checker.NewCycles()
}

func (c *Container) provideSpecImportsChecker() *checker.Imports {
	return checker.NewImport()
}

func (c *Container) provideSpecVisibilityChecker() *checker.Visibility {
	return checker.NewVisibility()
}

func (c *Container) provideSpecDeepScanChecker() *checker.DeepScan {
	return checker.NewDeepScan(
		c.provideReferenceRender(),
	)
}

func (c *Container) provideProjectFilesResolver() *resolver.Resolver {
	return resolver.NewResolver(
		c.provideProjectFilesScanner(),
		c.provideProjectFilesHolder(),
	)
}

func (c *Container) provideProjectFilesScanner() *scanner.Scanner {
	return scanner.NewScanner()
}

func (c *Container) provideProjectFilesHolder() *holder.Holder {
	return holder.NewHolder()
}

func (c *Container) provideProjectInfoAssembler() *info.Assembler {
	return info.NewAssembler()
}
