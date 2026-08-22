package assembler

import (
	"regexp"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/spec"
)

type excludeFilesMatcherAssembler struct{}

func newExcludeFilesMatcherAssembler() *excludeFilesMatcherAssembler {
	return &excludeFilesMatcherAssembler{}
}

func (efa *excludeFilesMatcherAssembler) assemble(spec *arch.Spec, yamlSpec spec.Document) error {
	for _, regString := range yamlSpec.ExcludedFilesRegExp() {
		matcher, err := regexp.Compile(regString.Value)
		if err != nil {
			continue
		}

		spec.ExcludeFilesMatcher = append(spec.ExcludeFilesMatcher, domain.NewReferable(
			matcher,
			regString.Reference,
		))
	}

	return nil
}
