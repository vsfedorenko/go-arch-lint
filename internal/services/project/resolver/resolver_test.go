package resolver

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// resolver_test.go pins the project-files resolver composition: the scan
// directory joins root + working directory (path.Clean semantics), exclude
// paths and file matchers unwrap from their Referable envelopes, scan
// failures wrap, and the holder attaches files to components.

type resWired struct {
	r      *Resolver
	scan   *mockprojectFilesResolver
	holder *mockprojectFilesHolder
}

func wireResolver(t *testing.T) *resWired {
	t.Helper()
	w := &resWired{
		scan:   newMockprojectFilesResolver(t),
		holder: newMockprojectFilesHolder(t),
	}
	w.r = NewResolver(w.scan, w.holder)
	return w
}

func resSpec(root, workdir string) arch.Spec {
	return arch.Spec{
		RootDirectory:    domain.NewReferable(root, domain.NewEmptyReference()),
		WorkingDirectory: domain.NewReferable(workdir, domain.NewEmptyReference()),
		ModuleName:       domain.NewReferable("example.com/m", domain.NewEmptyReference()),
	}
}

// The scan directory is root+workdir cleaned; excludes unwrap out of
// their Referable envelopes; the holder output is returned as-is.
func TestProjectFiles_ComposesScanAndHold(t *testing.T) {
	w := wireResolver(t)

	spec := resSpec("/project", "internal")
	spec.Exclude = []domain.Referable[models.ResolvedPath]{
		domain.NewReferable(models.ResolvedPath{ImportPath: "example.com/m/skip"}, domain.NewEmptyReference()),
	}
	matcher := regexp.MustCompile(`^.*_test\.go$`)
	spec.ExcludeFilesMatcher = []domain.Referable[*regexp.Regexp]{
		domain.NewReferable(matcher, domain.NewEmptyReference()),
	}
	files := []models.ProjectFile{{Path: "internal/a.go"}}
	wants := []models.FileHold{{File: models.ProjectFile{Path: "internal/a.go"}}}

	w.scan.EXPECT().ScanInWorkspace(
		mock.Anything,
		[]domain.WorkspaceModule(nil),
		"/project/internal",
		"example.com/m",
		[]models.ResolvedPath{{ImportPath: "example.com/m/skip"}},
		[]*regexp.Regexp{matcher},
	).Return(files, nil)
	w.holder.EXPECT().HoldProjectFiles(files, spec.Components).Return(wants)

	got, err := w.r.ProjectFiles(context.Background(), spec)
	require.NoError(t, err)
	assert.Equal(t, wants, got)
}

// workdir "." stays "." — the join must not produce "/project/." noise.
func TestProjectFiles_WorkdirDotCleans(t *testing.T) {
	w := wireResolver(t)
	spec := resSpec("/project", ".")

	w.scan.EXPECT().ScanInWorkspace(
		mock.Anything, mock.Anything, "/project", mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)
	w.holder.EXPECT().HoldProjectFiles(mock.Anything, mock.Anything).Return(nil)

	_, err := w.r.ProjectFiles(context.Background(), spec)
	require.NoError(t, err)
}

// Scan failures surface with the operation's wrap message.
func TestProjectFiles_ScanErrorWraps(t *testing.T) {
	w := wireResolver(t)
	w.scan.EXPECT().ScanInWorkspace(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("walk failed"))

	_, err := w.r.ProjectFiles(context.Background(), resSpec("/project", "internal"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to resolve project files")
}
