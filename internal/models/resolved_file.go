package models

import (
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

const (
	ImportTypeStdLib ImportType = iota
	ImportTypeProject
	ImportTypeVendor
)

type (
	ImportType uint8

	FileHold struct {
		File        ProjectFile
		ComponentID *string
	}

	ProjectFile struct {
		Path    string
		Imports []ResolvedImport

		// PackageName is the declared `package X` name of the file (used by
		// naming-convention rules).
		PackageName string
	}

	ResolvedImport struct {
		Name       string
		ImportType ImportType
		Reference  domain.Reference
	}
)

// GetImportType classifies an import path as std, project, or vendor
func GetImportType(importPath string, moduleName string, stdPackages map[string]struct{}) ImportType {
	return GetImportTypeInWorkspace(importPath, moduleName, nil, stdPackages)
}

// GetImportTypeInWorkspace classifies an import path against the root module
// PLUS the sibling modules of a go.work: an import of a workspace member is
// project code, not a vendor dependency. Without workspace modules it is
// identical to GetImportType.
func GetImportTypeInWorkspace(importPath string, moduleName string, workspaceModules []string, stdPackages map[string]struct{}) ImportType {
	if _, ok := stdPackages[importPath]; ok {
		return ImportTypeStdLib
	}

	if isModuleOrSubpackage(importPath, moduleName) {
		return ImportTypeProject
	}

	for _, modulePath := range workspaceModules {
		if isModuleOrSubpackage(importPath, modulePath) {
			return ImportTypeProject
		}
	}

	return ImportTypeVendor
}

// isModuleOrSubpackage reports whether importPath is exactly the module or a
// package below it. A straight prefix match is wrong: module "example.com/foo/bar"
// must not match "example.com/foo/bar-utils".
func isModuleOrSubpackage(importPath, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}
