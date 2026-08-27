package checker

import (
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
)

// ImportVerdict explains the allow/deny decision for one import against
// one component: the decision, the rule that produced it, and a concrete
// spec edit that would flip a denial into an allow (empty when allowed).
//
// It is the single source of truth for the decision: the imports checker
// consumes the boolean, the `explain` command renders the full story.
// Keeping both on one code path means an explained verdict can never
// diverge from what `check` actually enforces.
type ImportVerdict struct {
	Allowed bool
	Rule    string
	Fix     string
}

// VerdictForImport decides whether component may import resolvedImport
// under spec's global allow flags. The returned verdict carries the
// naming rule and, for denials, the fix.
func VerdictForImport(
	component arch.Component,
	resolvedImport models.ResolvedImport,
	allowDepOnAnyVendor bool,
) (ImportVerdict, error) {
	switch resolvedImport.ImportType {
	case models.ImportTypeStdLib:
		return ImportVerdict{
			Allowed: true,
			Rule:    "std library imports are always allowed",
		}, nil

	case models.ImportTypeVendor:
		if allowDepOnAnyVendor {
			return ImportVerdict{
				Allowed: true,
				Rule:    "allow.depOnAnyVendor = true (every vendor import allowed)",
			}, nil
		}

		return checkVendorImportVerdict(component, resolvedImport)

	case models.ImportTypeProject:
		return checkProjectImportVerdict(component, resolvedImport), nil

	default:
		panic(fmt.Sprintf("unknown import type: %+v", resolvedImport))
	}
}

func checkVendorImportVerdict(
	component arch.Component,
	resolvedImport models.ResolvedImport,
) (ImportVerdict, error) {
	if component.SpecialFlags.AllowAllVendorDeps.Value {
		return ImportVerdict{
			Allowed: true,
			Rule:    "vendor deps fully allowed for this component",
		}, nil
	}

	for _, vendorGlob := range component.AllowedVendorGlobs {
		matched, err := vendorGlob.Value.Match(resolvedImport.Name)
		if err != nil {
			return ImportVerdict{}, models.NewReferableErr(
				fmt.Errorf("invalid vendor glob '%s': %w",
					string(vendorGlob.Value),
					err,
				),
				vendorGlob.Reference,
			)
		}

		if matched {
			return ImportVerdict{
				Allowed: true,
				Rule:    fmt.Sprintf("vendor glob %q (Use of a declared Vendor)", vendorGlob.Value),
			}, nil
		}
	}

	return ImportVerdict{
		Allowed: false,
		Rule:    "no matching vendor rule: the import is not covered by any Vendor used by this component",
		Fix: fmt.Sprintf(
			"declare the dependency and allow it: vendor := Vendor(\"<name>\", %q) in arch.go, then Use(vendor) inside this component's Path(...)",
			resolvedImport.Name,
		),
	}, nil
}

func checkProjectImportVerdict(
	component arch.Component,
	resolvedImport models.ResolvedImport,
) ImportVerdict {
	if component.SpecialFlags.AllowAllProjectDeps.Value {
		return ImportVerdict{
			Allowed: true,
			Rule:    "all project deps allowed for this component",
		}
	}

	for _, allowedImportRef := range component.AllowedProjectImports {
		allowedImport := allowedImportRef.Value

		if allowedImport.ImportPath == resolvedImport.Name {
			return ImportVerdict{
				Allowed: true,
				Rule: fmt.Sprintf(
					"path %q is part of this component or of a component it Uses",
					allowedImport.LocalPath,
				),
			}
		}
	}

	return ImportVerdict{
		Allowed: false,
		Rule:    "no matching project rule: the import belongs to another component this one does not Use",
		Fix: fmt.Sprintf(
			"allow the dependency: Use(<component owning %q>) inside this component's Path(...)",
			resolvedImport.Name,
		),
	}
}
