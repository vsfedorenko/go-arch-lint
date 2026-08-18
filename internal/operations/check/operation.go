package check

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/services/baseline"
)

type (
	Operation struct {
		projectInfoAssembler projectInfoAssembler
		specAssembler        specAssembler
		specChecker          specChecker
		referenceRender      referenceRender
		highlightCodePreview bool
	}

	limiterResult struct {
		results      models.CheckResult
		omittedCount int
	}
)

func NewOperation(
	projectInfoAssembler projectInfoAssembler,
	specAssembler specAssembler,
	specChecker specChecker,
	referenceRender referenceRender,
	highlightCodePreview bool,
) *Operation {
	return &Operation{
		projectInfoAssembler: projectInfoAssembler,
		specAssembler:        specAssembler,
		specChecker:          specChecker,
		referenceRender:      referenceRender,
		highlightCodePreview: highlightCodePreview,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdCheckIn) (models.CmdCheckOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		// An unreadable project (missing go.mod, bad path) is a configuration
		// error: the check could not run at all. ExitCode maps this to 2 and
		// IsConfigError(err) must agree — a plain wrap would classify as a
		// system error and break the documented contract.
		return models.CmdCheckOut{}, models.NewConfigError(
			fmt.Sprintf("failed to assemble project info: %s", err),
		)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdCheckOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	result := models.CheckResult{}
	if len(spec.Integrity.DocumentNotices) == 0 {
		result, err = o.specChecker.Check(ctx, spec)
		if err != nil {
			return models.CmdCheckOut{}, fmt.Errorf("failed to check project deps: %w", err)
		}
	}

	// Baseline mode: record the full fingerprint set, or filter known
	// violations out so only NEW ones reach the renderer and the exit
	// code. Applied BEFORE limiting so the display cap never decides
	// what gets baselined or compared.
	knownCount := 0
	newCount := 0
	if in.BaselinePath != "" {
		if in.BaselineUpdate {
			if err := baseline.Save(in.BaselinePath, baseline.FromResult(result)); err != nil {
				return models.CmdCheckOut{}, models.NewConfigError(err.Error())
			}
		} else {
			base, exists, err := baseline.Load(in.BaselinePath)
			switch {
			case err != nil:
				return models.CmdCheckOut{}, models.NewConfigError(err.Error())
			case !exists:
				return models.CmdCheckOut{}, models.NewConfigError(fmt.Sprintf(
					"baseline file %s not found — record it first with --baseline-update %s",
					in.BaselinePath, in.BaselinePath,
				))
			}
			result, knownCount = baseline.FilterResult(result, base)
			newCount = len(result.DependencyWarnings) + len(result.MatchWarnings) +
				len(result.DeepscanWarnings) + len(result.NamingWarnings)
		}
	}

	limitedResult := o.limitResults(result, in.MaxWarnings)

	model := models.CmdCheckOut{
		ModuleName:             spec.ModuleName.Value,
		DocumentNotices:        o.assembleNotice(spec.Integrity),
		ArchHasWarnings:        limitedResult.results.HasNotices(),
		ArchWarningsDependency: limitedResult.results.DependencyWarnings,
		ArchWarningsMatch:      limitedResult.results.MatchWarnings,
		ArchWarningsDeepScan:   limitedResult.results.DeepscanWarnings,
		ArchWarningsNaming:     limitedResult.results.NamingWarnings,
		OmittedCount:           limitedResult.omittedCount,
		SuppressedCount:        limitedResult.results.SuppressedCount,
		BaselineKnownCount:     knownCount,
		BaselineNewCount:       newCount,
		Qualities: []models.CheckQuality{
			{
				ID:   "component_imports",
				Name: "Base: component imports",
				Used: len(spec.Components) > 0,
				Hint: "always on",
			},
			{
				ID:   "vendor_imports",
				Name: "Advanced: vendor imports",
				Used: !spec.Allow.DepOnAnyVendor.Value,
				Hint: "switch 'allow.depOnAnyVendor = false' (or delete) to on",
			},
			{
				ID:   "deepscan",
				Name: "Advanced: method calls and dependency injections",
				Used: spec.Allow.DeepScan.Value,
				Hint: "switch 'allow.deepScan = true' (or delete) to on",
			},
			{
				ID:   "naming",
				Name: "Base: package naming conventions",
				Used: spec.Naming != nil,
				Hint: "declare Naming(func(){ ForbiddenPackages(...) }) to on",
			},
			{
				ID:   "interface_placement",
				Name: "Base: interface placement (ports live with consumer)",
				Used: spec.InterfacePlacement != nil,
				Hint: "declare Interfaces(func(){ MustLiveWithConsumer() }) to on",
			},
			{
				ID:   "visibility",
				Name: "Base: export visibility rules",
				Used: spec.Visibility != nil,
				Hint: "declare Visibility(func(){ VisibleTo(...) }) to on",
			},
		},
	}

	if model.ArchHasWarnings {
		// violations found — exit code 1
		return model, models.NewUserSpaceError("check not successful")
	}

	if len(model.DocumentNotices) > 0 {
		// the spec itself is invalid (bad globs, unknown components, …) —
		// the check could not run, so this is a config error: exit code 2
		return model, models.NewConfigError("arch spec is invalid")
	}

	return model, nil
}

func (o *Operation) limitResults(result models.CheckResult, maxWarnings int) limiterResult {
	passCount := 0
	limitedResults := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{},
		MatchWarnings:      []models.CheckArchWarningMatch{},
		DeepscanWarnings:   []models.CheckArchWarningDeepscan{},
		NamingWarnings:     []models.CheckArchWarningNaming{},
		// Metadata (e.g. the suppression count) must survive limiting.
		SuppressedCount: result.SuppressedCount,
	}

	// append deps
	for _, notice := range result.DependencyWarnings {
		if passCount >= maxWarnings {
			break
		}

		limitedResults.DependencyWarnings = append(limitedResults.DependencyWarnings, notice)
		passCount++
	}

	// append not matched
	for _, notice := range result.MatchWarnings {
		if passCount >= maxWarnings {
			break
		}

		limitedResults.MatchWarnings = append(limitedResults.MatchWarnings, notice)
		passCount++
	}

	// append deep scan
	for _, notice := range result.DeepscanWarnings {
		if passCount >= maxWarnings {
			break
		}

		limitedResults.DeepscanWarnings = append(limitedResults.DeepscanWarnings, notice)
		passCount++
	}

	// append naming
	for _, notice := range result.NamingWarnings {
		if passCount >= maxWarnings {
			break
		}

		limitedResults.NamingWarnings = append(limitedResults.NamingWarnings, notice)
		passCount++
	}

	totalCount := 0 +
		len(result.DeepscanWarnings) +
		len(result.DependencyWarnings) +
		len(result.MatchWarnings) +
		len(result.NamingWarnings)

	return limiterResult{
		results:      limitedResults,
		omittedCount: totalCount - passCount,
	}
}

func (o *Operation) assembleNotice(integrity arch.Integrity) []models.CheckNotice {
	notices := make([]arch.Notice, 0)
	notices = append(notices, integrity.DocumentNotices...)

	results := make([]models.CheckNotice, 0)
	for _, notice := range notices {
		results = append(results, models.CheckNotice{
			Text:   fmt.Sprintf("%s", notice.Notice),
			File:   notice.Ref.File,
			Line:   notice.Ref.Line,
			Column: notice.Ref.Column,
			SourceCodePreview: o.referenceRender.SourceCode(
				notice.Ref.ExtendRange(1, 1),
				o.highlightCodePreview,
				true,
			),
		})
	}

	slices.SortFunc(results, func(a, b models.CheckNotice) int {
		if c := cmp.Compare(a.File, b.File); c != 0 {
			return c
		}
		return cmp.Compare(a.Line, b.Line)
	})

	return results
}
