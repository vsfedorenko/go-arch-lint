package holder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// Common test fixture paths.
const (
	pathProjectHolder  = "/app/internal/glue/project/holder"
	pathProjectPackage = "/app/internal/glue/project/package"
)

func Test_packageMathPath(t *testing.T) {
	type args struct {
		packagePath string
		path        string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exactly",
			args: args{
				packagePath: pathProjectHolder,
				path:        pathProjectHolder,
			},
			want: true,
		},
		{
			name: "subfolder",
			args: args{
				packagePath: pathProjectHolder,
				path:        "/app/internal/glue/project/holder/sub",
			},
			want: false,
		},
		{
			name: "subfolder 2",
			args: args{
				packagePath: pathProjectHolder,
				path:        "/app/internal/glue/project/holder/sub/b",
			},
			want: false,
		},
		{
			name: "lower 1",
			args: args{
				packagePath: pathProjectHolder,
				path:        "/app/internal/glue/project",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packageMathPath(tt.args.packagePath, tt.args.path)
			assert.Equal(t, tt.want, got, "packageMathPath()")
		})
	}
}

func Test_componentMatchPackage(t *testing.T) {
	type args struct {
		packagePath string
		component   arch.Component
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "match",
			args: args{
				packagePath: pathProjectPackage,
				component: arch.Component{
					ResolvedPaths: []domain.Referable[models.ResolvedPath]{
						domain.NewReferable(
							models.ResolvedPath{AbsPath: pathProjectPackage},
							domain.NewEmptyReference(),
						),
					},
				},
			},
			want: true,
		},
		{
			name: "not match",
			args: args{
				packagePath: pathProjectPackage,
				component: arch.Component{
					ResolvedPaths: []domain.Referable[models.ResolvedPath]{
						domain.NewReferable(
							models.ResolvedPath{AbsPath: "/app/internal/glue/project/package/sub"},
							domain.NewEmptyReference(),
						),
					},
				},
			},
			want: false,
		},
		{
			name: "any match",
			args: args{
				packagePath: pathProjectPackage,
				component: arch.Component{
					ResolvedPaths: []domain.Referable[models.ResolvedPath]{
						domain.NewReferable(
							models.ResolvedPath{AbsPath: "/app/internal/glue/project/package/sub"},
							domain.NewEmptyReference(),
						),
						domain.NewReferable(
							models.ResolvedPath{AbsPath: pathProjectPackage},
							domain.NewEmptyReference(),
						),
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := componentMatchPackage(tt.args.packagePath, tt.args.component)
			assert.Equal(t, tt.want, got, "componentMatchPackage()")
		})
	}
}

func Test_componentsMatchesFile(t *testing.T) {
	type args struct {
		filePath   string
		components []arch.Component
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "s1",
			args: args{
				filePath: "/app/file.go",
				components: []arch.Component{
					{
						Name: domain.NewReferable("A", domain.NewEmptyReference()),
						ResolvedPaths: []domain.Referable[models.ResolvedPath]{
							domain.NewReferable(
								models.ResolvedPath{AbsPath: "/app"},
								domain.NewEmptyReference(),
							),
						},
					},
					{
						Name: domain.NewReferable("C", domain.NewEmptyReference()),
						ResolvedPaths: []domain.Referable[models.ResolvedPath]{
							domain.NewReferable(
								models.ResolvedPath{AbsPath: "/app/sub"},
								domain.NewEmptyReference(),
							),
						},
					},
					{
						Name: domain.NewReferable("D", domain.NewEmptyReference()),
						ResolvedPaths: []domain.Referable[models.ResolvedPath]{
							domain.NewReferable(
								models.ResolvedPath{AbsPath: "/"},
								domain.NewEmptyReference(),
							),
						},
					},
					{
						Name: domain.NewReferable("B", domain.NewEmptyReference()),
						ResolvedPaths: []domain.Referable[models.ResolvedPath]{
							domain.NewReferable(
								models.ResolvedPath{AbsPath: "/app"},
								domain.NewEmptyReference(),
							),
						},
					},
				},
			},
			want: []string{"A", "B"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := componentsMatchesFile(tt.args.filePath, tt.args.components)
			assert.Equal(t, tt.want, got, "componentsMatchesFile()")
		})
	}
}

func Test_compare(t *testing.T) {
	type args struct {
		a matchedComponent
		b matchedComponent
	}
	tests := []struct {
		name      string
		args      args
		bIsBetter bool
	}{
		{
			name: "count better A",
			args: args{
				a: matchedComponent{id: "A", filesCount: 3},
				b: matchedComponent{id: "B", filesCount: 4},
			},
			bIsBetter: false,
		},
		{
			name: "count better B",
			args: args{
				a: matchedComponent{id: "A", filesCount: 4},
				b: matchedComponent{id: "B", filesCount: 3},
			},
			bIsBetter: true,
		},
		{
			name: "more specified, better A",
			args: args{
				a: matchedComponent{id: "/a/b/c/d", filesCount: 3},
				b: matchedComponent{id: "/a/b/c", filesCount: 3},
			},
			bIsBetter: false,
		},
		{
			name: "more specified, better B",
			args: args{
				a: matchedComponent{id: "/a/b/c", filesCount: 3},
				b: matchedComponent{id: "/a/b/c/d", filesCount: 3},
			},
			bIsBetter: true,
		},
		{
			name: "longer name, better A",
			args: args{
				a: matchedComponent{id: "/a/b/aaaa", filesCount: 3},
				b: matchedComponent{id: "/a/b/bbb", filesCount: 3},
			},
			bIsBetter: false,
		},
		{
			name: "longer name, better B",
			args: args{
				a: matchedComponent{id: "/a/b/bbb", filesCount: 3},
				b: matchedComponent{id: "/a/b/aaaa", filesCount: 3},
			},
			bIsBetter: true,
		},
		{
			name: "stable sort, better A",
			args: args{
				a: matchedComponent{id: "/aaa", filesCount: 3},
				b: matchedComponent{id: "/bbb", filesCount: 3},
			},
			bIsBetter: false,
		},
		{
			name: "stable sort, better B",
			args: args{
				a: matchedComponent{id: "/bbb", filesCount: 3},
				b: matchedComponent{id: "/aaa", filesCount: 3},
			},
			bIsBetter: true,
		},
		{
			name: "equal, better always A",
			args: args{
				a: matchedComponent{id: "/file/src.go", filesCount: 3},
				b: matchedComponent{id: "/file/src.go", filesCount: 3},
			},
			bIsBetter: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compare(tt.args.a, tt.args.b)
			assert.Equal(t, tt.bIsBetter, got, "compare()")
		})
	}
}
