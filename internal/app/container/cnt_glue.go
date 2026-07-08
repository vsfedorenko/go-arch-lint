package container

import (
	"github.com/vsfedorenko/go-arch-lint/internal/services/checker"
	"github.com/vsfedorenko/go-arch-lint/internal/services/common/path"
	"github.com/vsfedorenko/go-arch-lint/internal/services/project/holder"
	"github.com/vsfedorenko/go-arch-lint/internal/services/project/info"
	"github.com/vsfedorenko/go-arch-lint/internal/services/project/resolver"
	"github.com/vsfedorenko/go-arch-lint/internal/services/project/scanner"
	"github.com/vsfedorenko/go-arch-lint/internal/services/render/code"
	specassembler "github.com/vsfedorenko/go-arch-lint/internal/services/spec/assembler"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec/decoder"
	specvalidator "github.com/vsfedorenko/go-arch-lint/internal/services/spec/validator"
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

func (c *Container) provideGoSpecProvider() *decoder.GoDecoder {
	return decoder.NewGoDecoder(c.specBuilder)
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
		c.provideSpecImportsChecker(),
		c.provideSpecDeepScanChecker(),
	)
}

func (c *Container) provideSpecImportsChecker() *checker.Imports {
	return checker.NewImport(
		c.provideProjectFilesResolver(),
	)
}

func (c *Container) provideSpecDeepScanChecker() *checker.DeepScan {
	return checker.NewDeepScan(
		c.provideProjectFilesResolver(),
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
