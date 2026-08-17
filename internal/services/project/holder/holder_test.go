package holder

import (
	"reflect"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
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
			if got := packageMathPath(tt.args.packagePath, tt.args.path); got != tt.want {
				t.Errorf("packageMathPath() = %v, want %v", got, tt.want)
			}
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
			if got := componentMatchPackage(tt.args.packagePath, tt.args.component); got != tt.want {
				t.Errorf("componentMatchPackage() = %v, want %v", got, tt.want)
			}
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
			if got := componentsMatchesFile(tt.args.filePath, tt.args.components); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("componentsMatchesFile() = %v, want %v", got, tt.want)
			}
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
			if got := compare(tt.args.a, tt.args.b); got != tt.bIsBetter {
				t.Errorf("compare() = %v, want %v", got, tt.bIsBetter)
			}
		})
	}
}
