package assembler

import (
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
)

// tiersAssembler copies the ordered tier list from the document into the
// assembled spec. Validation (unknown components, duplicates) happens in
// the DSL builders at spec-build time; here the values are already sound.
type tiersAssembler struct{}

func newTiersAssembler() *tiersAssembler {
	return &tiersAssembler{}
}

func (a *tiersAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	tiers := document.Tiers()
	if len(tiers) == 0 {
		return nil
	}

	spec.Tiers = make([]arch.Tier, len(tiers))
	for i, tier := range tiers {
		spec.Tiers[i] = arch.Tier{
			Name:       tier.Name,
			Components: append([]string(nil), tier.Components...),
			Reference:  tier.Reference,
		}
	}

	return nil
}

// assembleNaming copies packaging-convention rules into the spec.
type namingAssembler struct{}

func newNamingAssembler() *namingAssembler {
	return &namingAssembler{}
}

func (a *namingAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	naming := document.Naming()
	if naming == nil {
		return nil
	}

	forbidden := naming.ForbiddenPackages()
	if len(forbidden) == 0 {
		return nil
	}

	spec.Naming = &arch.Naming{
		ForbiddenPackages: append(
			[]domain.Referable[string](nil), forbidden...,
		),
	}

	return nil
}

// assembleInterfacePlacement copies interface-location rules into the spec.
type interfacePlacementAssembler struct{}

func newInterfacePlacementAssembler() *interfacePlacementAssembler {
	return &interfacePlacementAssembler{}
}

func (a *interfacePlacementAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	placement := document.InterfacePlacement()
	if placement == nil || !placement.MustLiveWithConsumer() {
		return nil
	}

	spec.InterfacePlacement = &arch.InterfacePlacement{
		MustLiveWithConsumer: true,
	}

	return nil
}

type visibilityAssembler struct{}

func newVisibilityAssembler() *visibilityAssembler {
	return &visibilityAssembler{}
}

func (a *visibilityAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	visibility := document.Visibility()
	if visibility == nil {
		return nil
	}

	rules := visibility.Rules()
	if len(rules) == 0 {
		return nil
	}

	archRules := make([]arch.VisibilityRule, len(rules))
	for i, rule := range rules {
		archRules[i] = arch.VisibilityRule{
			Component: rule.Component,
			Allowed:   append([]string(nil), rule.Allowed...),
			Reference: rule.Reference,
		}
	}

	spec.Visibility = &arch.Visibility{Rules: archRules}
	return nil
}
