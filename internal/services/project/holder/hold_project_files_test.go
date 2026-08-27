package holder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// componentFixture builds a component matching the given package directories.
func componentFixture(name string, paths ...string) arch.Component {
	component := arch.Component{
		Name: domain.NewReferable(name, domain.NewEmptyReference()),
	}
	for _, path := range paths {
		component.ResolvedPaths = append(component.ResolvedPaths, domain.NewReferable(
			models.ResolvedPath{AbsPath: path},
			domain.NewEmptyReference(),
		))
	}

	return component
}

// fileFixtures builds project files declared at the given paths.
func fileFixtures(paths ...string) []models.ProjectFile {
	files := make([]models.ProjectFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, models.ProjectFile{Path: path})
	}

	return files
}

// holdByID runs HoldProjectFiles and indexes the results by file path so
// assertions do not depend on map iteration order.
func holdByID(t *testing.T, files []models.ProjectFile, components []arch.Component) map[string]*string {
	t.Helper()

	byID := make(map[string]*string, len(files))
	for _, hold := range NewHolder().HoldProjectFiles(files, components) {
		byID[hold.File.Path] = hold.ComponentID
	}

	return byID
}

func TestHoldProjectFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      []models.ProjectFile
		components []arch.Component
		want       map[string]*string // nil value = no component
	}{
		{
			name:  "empty input yields no holds",
			files: nil,
			components: []arch.Component{
				componentFixture("/app", "/app"),
			},
			want: map[string]*string{},
		},
		{
			name:  "unmatched file yields nil component",
			files: fileFixtures("/lonely.go"),
			components: []arch.Component{
				componentFixture("/app", "/app"),
			},
			want: map[string]*string{
				"/lonely.go": nil,
			},
		},
		{
			name:  "single match assigns the component",
			files: fileFixtures("/app/main.go"),
			components: []arch.Component{
				componentFixture("/app", "/app"),
			},
			want: map[string]*string{
				"/app/main.go": ptr("/app"),
			},
		},
		{
			name: "narrower component wins over broader",
			files: fileFixtures(
				"/app/main.go",
				"/app/internal/glue/glue.go",
			),
			components: []arch.Component{
				componentFixture("/app", "/app"),                             // matches 2 files
				componentFixture("/app/internal/glue", "/app/internal/glue"), // matches 1 file
			},
			want: map[string]*string{
				"/app/main.go":               ptr("/app"),
				"/app/internal/glue/glue.go": ptr("/app/internal/glue"),
			},
		},
		{
			name: "equal file counts resolve to the deeper path",
			files: fileFixtures(
				"/a/b/c/f.go",
				"/x/y/other.go",
			),
			components: []arch.Component{
				componentFixture("/a", "/a"),         // count 1
				componentFixture("/a/b/c", "/a/b/c"), // count 1, deeper
				componentFixture("/x/y", "/x/y"),
			},
			want: map[string]*string{
				"/a/b/c/f.go":   ptr("/a/b/c"),
				"/x/y/other.go": ptr("/x/y"),
			},
		},
		{
			name:  "file in a subpackage of a component dir is not matched",
			files: fileFixtures("/app/sub/nested.go"),
			components: []arch.Component{
				componentFixture("/app", "/app"),
			},
			want: map[string]*string{
				"/app/sub/nested.go": nil,
			},
		},
		{
			name: "component with several resolved paths matches any of them",
			files: fileFixtures(
				"/svc/a.go",
				"/lib/b.go",
			),
			components: []arch.Component{
				componentFixture("shared", "/svc", "/lib"),
			},
			want: map[string]*string{
				"/svc/a.go": ptr("shared"),
				"/lib/b.go": ptr("shared"),
			},
		},
		{
			name: "overlap resolves to the component matching fewer files",
			files: fileFixtures(
				"/shared/f.go",
				"/uniq_a/g.go",
			),
			components: []arch.Component{
				componentFixture("broad", "/shared", "/uniq_a"), // matches 2 files
				componentFixture("narrow", "/shared"),           // matches 1 file
			},
			want: map[string]*string{
				"/shared/f.go": ptr("narrow"),
				"/uniq_a/g.go": ptr("broad"),
			},
		},
		{
			name:  "full tie keeps the lexicographically smaller component id",
			files: fileFixtures("/same/f.go"),
			components: []arch.Component{
				componentFixture("bbb", "/same"),
				componentFixture("aaa", "/same"),
			},
			want: map[string]*string{
				"/same/f.go": ptr("aaa"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := holdByID(t, tt.files, tt.components)
			assert.Equal(t, tt.want, got, "HoldProjectFiles()")
		})
	}
}

// ptr is a tiny helper for want fixtures (values are addressable only via
// vars, so a function reads cleaner than three var blocks).
func ptr(value string) *string {
	return &value
}
