package selfInspect

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// operation_test.go pins the selfInspect operation at the unit layer:
// module/root/version surfacing, document notices vs suggestions as
// separate annotation lists, and error wrapping from both ports.

type selfWired struct {
	op    *Operation
	info  *mockprojectInfoAssembler
	specA *mockspecAssembler
}

func wireSelf(t *testing.T) *selfWired {
	t.Helper()
	w := &selfWired{
		info:  newMockprojectInfoAssembler(t),
		specA: newMockspecAssembler(t),
	}
	w.op = NewOperation(w.specA, w.info, "v2.4.1")
	return w
}

func selfSpec(notices, suggestions []arch.Notice) arch.Spec {
	spec := arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable("example.com/m", domain.NewEmptyReference()),
	}
	spec.Integrity.DocumentNotices = notices
	spec.Integrity.Suggestions = suggestions
	return spec
}

// Happy path: module and directory come from the PROJECT info, the linter
// version from the operation wiring; notices and suggestions stay
// separate lists.
func TestBehave_SurfacesProjectAndAnnotations(t *testing.T) {
	w := wireSelf(t)
	w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{
		ModuleName: "example.com/m",
		Directory:  "/project",
	}, nil)
	w.specA.EXPECT().Assemble(mock.Anything).Return(selfSpec(
		[]arch.Notice{{Notice: errors.New("unknown component 'x'")}},
		[]arch.Notice{{Notice: errors.New("consider IgnoreNotFoundComponents")}},
	), nil)

	out, err := w.op.Behave(models.CmdSelfInspectIn{})
	require.NoError(t, err)
	assert.Equal(t, "example.com/m", out.ModuleName)
	assert.Equal(t, "/project", out.RootDirectory)
	assert.Equal(t, "v2.4.1", out.LinterVersion)

	require.Len(t, out.Notices, 1)
	assert.Equal(t, "unknown component 'x'", out.Notices[0].Text)
	require.Len(t, out.Suggestions, 1)
	assert.Equal(t, "consider IgnoreNotFoundComponents", out.Suggestions[0].Text)
}

// A spec with no integrity annotations yields EMPTY lists, not nils —
// the renderer contract for JSON consumers.
func TestBehave_CleanSpecEmptyAnnotations(t *testing.T) {
	w := wireSelf(t)
	w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
	w.specA.EXPECT().Assemble(mock.Anything).Return(selfSpec(nil, nil), nil)

	out, err := w.op.Behave(models.CmdSelfInspectIn{})
	require.NoError(t, err)
	assert.NotNil(t, out.Notices, "notices must be empty list, not nil")
	assert.NotNil(t, out.Suggestions, "suggestions must be empty list, not nil")
	assert.Empty(t, out.Notices)
	assert.Empty(t, out.Suggestions)
}

// Both port failures wrap their cause with the operation's message.
func TestBehave_PortErrorsWrap(t *testing.T) {
	t.Run("project info", func(t *testing.T) {
		w := wireSelf(t)
		w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
			Return(domain.Project{}, errors.New("no go.mod"))

		_, err := w.op.Behave(models.CmdSelfInspectIn{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to assemble project info")
	})

	t.Run("spec assembler", func(t *testing.T) {
		w := wireSelf(t)
		w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
		w.specA.EXPECT().Assemble(mock.Anything).Return(arch.Spec{}, errors.New("bad glob"))

		_, err := w.op.Behave(models.CmdSelfInspectIn{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed assemble spec")
	})
}
