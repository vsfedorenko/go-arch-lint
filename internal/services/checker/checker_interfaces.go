package checker

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
)

// InterfacePlacement asserts interface-location conventions. With
// MustLiveWithConsumer, an interface that is USED by exactly one other
// component must be DECLARED in that component — hexagonal ports live
// with the consumer, not next to the implementation.
//
// Analysis is syntax-only (go/parser, no type loading): selector usages
// `pkg.Iface` are resolved through the file's import block (explicit
// alias or the conventional last-path-segment package name). Interfaces
// consumed by 0 or 2+ components are allowed where they are; an
// interface with a single cross-component consumer must move.
type InterfacePlacement struct{}

func NewInterfacePlacement() *InterfacePlacement {
	return &InterfacePlacement{}
}

// interfaceDecl is a declared interface.
type interfaceDecl struct {
	name string
	pkg  string // declaring package directory (absolute path)
}

// one usage of an interface: consumer file + interface declaration.
type interfaceUsage struct {
	file  string
	iface interfaceDecl
}

func (c *InterfacePlacement) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	if spec.InterfacePlacement == nil || !spec.InterfacePlacement.MustLiveWithConsumer {
		return models.CheckResult{}, nil
	}

	// Package dir -> owning component (scanner's ownership rules).
	pkgOwner := buildPackageOwnerMap(projectFiles)

	usages, declFiles, err := scanInterfaceUsage(spec, projectFiles)
	if err != nil {
		return models.CheckResult{}, fmt.Errorf("failed to scan interface usage: %w", err)
	}

	// interface (by declaring pkg+name) -> consuming components -> witness.
	type ifaceKey struct{ pkg, name string }
	consumers := map[ifaceKey]map[string]string{}

	for _, usage := range usages {
		_, okDecl := pkgOwner[usage.iface.pkg]
		consOwner, okCons := pkgOwner[packagePathOf(usage.file)]
		if !okDecl || !okCons {
			continue // unknown ownership
		}

		// Count EVERY consuming component — including the declaring one.
		// A same-component cross-package usage (validator importing the
		// package that declares Document) means the interface serves its
		// own component too, so it is not a pure port for one external
		// consumer; the violation condition below requires the single
		// consumer to differ from the declaring component.
		key := ifaceKey{pkg: usage.iface.pkg, name: usage.iface.name}
		if consumers[key] == nil {
			consumers[key] = map[string]string{}
		}
		consumers[key][consOwner] = usage.file
	}

	type violation struct {
		iface    interfaceDecl
		consumer string
	}
	var violations []violation

	for key, comps := range consumers {
		if len(comps) != 1 {
			continue // 0 consumers or genuinely shared — allowed
		}

		var consumer string
		for comp := range comps {
			consumer = comp
		}

		if consumer == pkgOwner[key.pkg] {
			// The only consumer is the declaring component itself
			// (same component, other package) — internal interface.
			continue
		}

		violations = append(violations, violation{
			iface:    interfaceDecl{name: key.name, pkg: key.pkg},
			consumer: consumer,
		})
	}

	// Deterministic order: by interface name.
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].iface.name < violations[j].iface.name
	})

	result := models.CheckResult{}

	for _, v := range violations {
		declFile := declFiles[v.iface.pkg+"|"+v.iface.name]

		result.DependencyWarnings = append(result.DependencyWarnings, models.CheckArchWarningDependency{
			ComponentName: fmt.Sprintf(
				"interface '%s' must live with its consumer '%s' (declared in component '%s')",
				v.iface.name, v.consumer, pkgOwner[v.iface.pkg],
			),
			FileRelativePath:   strings.TrimPrefix(declFile, spec.RootDirectory.Value),
			FileAbsolutePath:   declFile,
			ResolvedImportName: v.iface.name,
		})
	}

	return result, nil
}

// scanInterfaceUsage parses every project file once (full syntax parse)
// and returns cross-package usages of declared interfaces plus a map of
// interface (pkg|name) -> declaring file.
func scanInterfaceUsage(spec arch.Spec, projectFiles []models.FileHold) ([]interfaceUsage, map[string]string, error) {
	fset := token.NewFileSet()

	// Pass 1: parse everything, index interfaces by package dir and keep
	// the per-file ASTs for the usage pass.
	type parsedFile struct {
		path string
		ast  *ast.File
	}
	parsed := make([]parsedFile, 0, len(projectFiles))

	ifacesByPkg := map[string]map[string]bool{} // pkg dir -> interface names
	declFiles := map[string]string{}            // pkg|name -> declaring file

	for _, hold := range projectFiles {
		fileAst, err := parser.ParseFile(fset, hold.File.Path, nil, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse '%s': %w", hold.File.Path, err)
		}

		parsed = append(parsed, parsedFile{path: hold.File.Path, ast: fileAst})

		pkg := packagePathOf(hold.File.Path)
		for _, decl := range fileAst.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, isIface := typeSpec.Type.(*ast.InterfaceType); isIface {
					if ifacesByPkg[pkg] == nil {
						ifacesByPkg[pkg] = map[string]bool{}
					}
					ifacesByPkg[pkg][typeSpec.Name.Name] = true
					declFiles[pkg+"|"+typeSpec.Name.Name] = hold.File.Path
				}
			}
		}
	}

	// importPath -> absolute package dir under the module root.
	modulePrefix := spec.ModuleName.Value + "/"
	rootDir := spec.RootDirectory.Value
	toDir := func(importPath string) (string, bool) {
		if !strings.HasPrefix(importPath, modulePrefix) {
			return "", false
		}
		rel := strings.TrimPrefix(importPath, modulePrefix)
		return rootDir + "/" + rel, true
	}

	var usages []interfaceUsage

	for _, pf := range parsed {
		// selectorName -> imported package dir for this file.
		selectorDirs := map[string]string{}
		for _, imp := range pf.ast.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			dir, ok := toDir(importPath)
			if !ok {
				continue // stdlib / vendor
			}

			name := "" // conventional package name = last path segment
			if segments := strings.Split(importPath, "/"); len(segments) > 0 {
				name = segments[len(segments)-1]
			}
			if imp.Name != nil { // explicit alias or dot/blank import
				name = imp.Name.Name
			}

			if name != "" && name != "." && name != "_" {
				selectorDirs[name] = dir
			}
		}

		ownPkg := packagePathOf(pf.path)

		ast.Inspect(pf.ast, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				ident, ok := node.X.(*ast.Ident)
				if !ok {
					return true
				}

				dir, imported := selectorDirs[ident.Name]
				if !imported || dir == ownPkg {
					return true
				}

				if ifacesByPkg[dir] != nil && ifacesByPkg[dir][node.Sel.Name] {
					usages = append(usages, interfaceUsage{
						file:  pf.path,
						iface: interfaceDecl{name: node.Sel.Name, pkg: dir},
					})
				}
			case *ast.TypeSpec:
				// The interface's own declaration — not a usage.
				return false
			case *ast.Ident:
				// Bare identifier referencing an interface declared in the
				// SAME package: the declaring component consumes its own
				// interface (e.g. container's RunCheck taking SpecDecoder).
				// Recorded so the declaring component counts as a consumer
				// — a port must serve ONLY the external consumer to require
				// relocation.
				if ifacesByPkg[ownPkg] != nil && ifacesByPkg[ownPkg][node.Name] {
					usages = append(usages, interfaceUsage{
						file:  pf.path,
						iface: interfaceDecl{name: node.Name, pkg: ownPkg},
					})
				}
			}
			return true
		})
	}

	return usages, declFiles, nil
}
