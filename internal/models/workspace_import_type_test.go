package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testRootModule    = "example.com/root"
	testSiblingModule = "example.com/y"
)

// workspace_import_type_test.go pins the workspace-aware import
// classification: an import of a go.work sibling module is PROJECT code,
// not a vendor dependency.

func TestGetImportTypeInWorkspace(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		rootModule string
		workspace  []string
		want       ImportType
	}{
		{
			name:       "root module package is project",
			importPath: "example.com/root/internal/a",
			rootModule: testRootModule,
			want:       ImportTypeProject,
		},
		{
			name:       "workspace member is project",
			importPath: "example.com/y/api",
			rootModule: testRootModule,
			workspace:  []string{testSiblingModule},
			want:       ImportTypeProject,
		},
		{
			name:       "workspace member exact match is project",
			importPath: "example.com/y",
			rootModule: testRootModule,
			workspace:  []string{testSiblingModule},
			want:       ImportTypeProject,
		},
		{
			name:       "module-like sibling suffix is NOT project",
			importPath: "example.com/y-utils",
			rootModule: testRootModule,
			workspace:  []string{testSiblingModule},
			want:       ImportTypeVendor,
		},
		{
			name:       "unknown module stays vendor",
			importPath: "github.com/spf13/cobra",
			rootModule: testRootModule,
			workspace:  []string{testSiblingModule},
			want:       ImportTypeVendor,
		},
		{
			name:       "std stays std with workspace present",
			importPath: "fmt",
			rootModule: testRootModule,
			workspace:  []string{testSiblingModule},
			want:       ImportTypeStdLib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetImportTypeInWorkspace(tt.importPath, tt.rootModule, tt.workspace, stdPackagesFixture())
			assert.Equal(t, tt.want, got)
		})
	}
}

func stdPackagesFixture() map[string]struct{} {
	return map[string]struct{}{"fmt": {}, "os": {}}
}
