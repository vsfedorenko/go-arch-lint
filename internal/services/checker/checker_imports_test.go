package checker

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

const (
	testModulePath = "github.com/vsfedorenko/go-arch-lint/checker/test"
)

func makeTestProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0) //nolint:dogsled // runtime.Caller returns 4 values; only the filename matters
	return filepath.Dir(filename) + "/test"
}

func makeTestAbsPath(localPath string) string {
	return makeTestProjectRoot() + "/" + localPath
}

func makeBool(b bool) domain.Referable[bool] {
	return domain.NewReferable(b, domain.NewEmptyReference())
}

func makeTestResolvedPath(localPath string) domain.Referable[models.ResolvedPath] {
	return domain.NewReferable(
		models.ResolvedPath{
			ImportPath: testModulePath + "/" + localPath,
			AbsPath:    makeTestAbsPath(localPath),
			LocalPath:  localPath,
		},
		domain.NewEmptyReference(),
	)
}

func makeTestResolvedProjectImport() models.ResolvedImport {
	return models.ResolvedImport{
		Name:       testModulePath + "/needle",
		ImportType: models.ImportTypeProject,
	}
}

func makeTestResolvedVendorImport(localPath string) models.ResolvedImport {
	return models.ResolvedImport{
		Name:       "github.com/vendor/lib/" + localPath,
		ImportType: models.ImportTypeVendor,
	}
}

func makeTestResolvedStdlibImport() models.ResolvedImport {
	return models.ResolvedImport{
		Name:       "fmt",
		ImportType: models.ImportTypeStdLib,
	}
}

func Test_checkImportPath(t *testing.T) {
	type args struct {
		componentImports []domain.Referable[models.ResolvedPath]
		resolvedImport   models.ResolvedImport
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "have needed import",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("needle"),
					makeTestResolvedPath("checker"),
					makeTestResolvedPath("path"),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "not have needed import",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("checker"),
					makeTestResolvedPath("path"),
					makeTestResolvedPath("some"),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
		{
			name: "empty",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				resolvedImport:   makeTestResolvedProjectImport(),
			},
			want: false,
		},
		{
			name: "only needed",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("needle"),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp := arch.Component{
				Name:                  domain.NewReferable("component", domain.NewEmptyReference()),
				AllowedProjectImports: tt.args.componentImports,
			}

			got := checkProjectImport(cmp, tt.args.resolvedImport)
			assert.Equal(t, tt.want, got, "checkImportPath()")
		})
	}
}

func Test_checkProjectImport(t *testing.T) {
	type args struct {
		componentImports []domain.Referable[models.ResolvedPath]
		componentFlags   arch.SpecialFlags
		resolvedImport   models.ResolvedImport
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no flags, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
		{
			name: "project flag, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(true),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "vendor flag, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(true),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
		{
			name: "flag + list exactly same",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("needle"),
				},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(true),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "flag + list have needle",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("some"),
					makeTestResolvedPath("needle"),
					makeTestResolvedPath("module12"),
				},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(true),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "flag + list not have needle",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("some"),
					makeTestResolvedPath("module12"),
				},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(true),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "no flag + list have needle",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("some"),
					makeTestResolvedPath("needle"),
					makeTestResolvedPath("module12"),
				},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "no flag + list not have needle",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{
					makeTestResolvedPath("some"),
					makeTestResolvedPath("module12"),
				},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp := arch.Component{
				Name:                  domain.NewReferable("component", domain.NewEmptyReference()),
				SpecialFlags:          tt.args.componentFlags,
				AllowedProjectImports: tt.args.componentImports,
			}

			got := checkProjectImport(cmp, tt.args.resolvedImport)
			assert.Equal(t, tt.want, got, "checkProjectImport()")
		})
	}
}

func Test_checkVendorImport(t *testing.T) {
	type args struct {
		componentImports []domain.Referable[models.ResolvedPath]
		componentFlags   arch.SpecialFlags
		resolvedImport   models.ResolvedImport
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no flags, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
		{
			name: "vendor flag, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(false),
					AllowAllVendorDeps:  makeBool(true),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: true,
		},
		{
			name: "project flag, empty list",
			args: args{
				componentImports: []domain.Referable[models.ResolvedPath]{},
				componentFlags: arch.SpecialFlags{
					AllowAllProjectDeps: makeBool(true),
					AllowAllVendorDeps:  makeBool(false),
				},
				resolvedImport: makeTestResolvedProjectImport(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp := arch.Component{
				Name:                  domain.NewReferable("component", domain.NewEmptyReference()),
				SpecialFlags:          tt.args.componentFlags,
				AllowedProjectImports: tt.args.componentImports,
			}

			got, err := checkVendorImport(cmp, tt.args.resolvedImport)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChecker_checkImport(t *testing.T) {
	type args struct {
		resolvedImport    models.ResolvedImport
		dependOnAnyVendor bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "allow any vendor / vendor",
			args: args{
				resolvedImport:    makeTestResolvedVendorImport("needle"),
				dependOnAnyVendor: true,
			},
			want: true,
		},
		{
			name: "not allow any vendor / vendor",
			args: args{
				resolvedImport:    makeTestResolvedVendorImport("needle"),
				dependOnAnyVendor: false,
			},
			want: false,
		},
		{
			name: "allow any vendor / project dep",
			args: args{
				resolvedImport:    makeTestResolvedProjectImport(),
				dependOnAnyVendor: true,
			},
			want: false,
		},
		{
			name: "stdlib always ok",
			args: args{
				resolvedImport:    makeTestResolvedStdlibImport(),
				dependOnAnyVendor: false,
			},
			want: true,
		},
		{
			name: "project reject",
			args: args{
				resolvedImport:    makeTestResolvedProjectImport(),
				dependOnAnyVendor: false,
			},
			want: false,
		},
		{
			name: "vendor reject",
			args: args{
				resolvedImport:    makeTestResolvedVendorImport("needle"),
				dependOnAnyVendor: false,
			},
			want: false,
		},
	}

	cmp := arch.Component{
		Name: domain.NewReferable("component", domain.NewEmptyReference()),
		SpecialFlags: arch.SpecialFlags{
			AllowAllProjectDeps: makeBool(false),
			AllowAllVendorDeps:  makeBool(false),
		},
		AllowedProjectImports: []domain.Referable[models.ResolvedPath]{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkImport(cmp, tt.args.resolvedImport, tt.args.dependOnAnyVendor)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("check unknown import type", func(t *testing.T) {
		assert.Panics(t, func() {
			resolvedImport := models.ResolvedImport{
				Name:       "something",
				ImportType: 100,
			}

			_, _ = checkImport(cmp, resolvedImport, false)
		})
	})
}
