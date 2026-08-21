package check

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// operation_test.go pins the check operation's branching at the unit layer
// with mocked ports (the wiring itself is covered by integration runs):
// config-error classification, baseline record/filter, the MaxWarnings cap
// (metadata must survive it), and the exit-code trichotomy
// nil / UserSpaceError / ConfigError.

// fixtures ---------------------------------------------------------------

func testSpec() arch.Spec {
	return arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable("example.com/m", domain.NewEmptyReference()),
	}
}

func depWarning(component string) models.CheckArchWarningDependency {
	return models.CheckArchWarningDependency{
		ComponentName:      component,
		FileRelativePath:   "internal/" + component + "/a.go",
		ResolvedImportName: "example.com/m/" + component + "dep",
	}
}

// wired assembles the operation with fresh EXPECT-driven mocks. The
// notice renderer is included so invalid-spec paths can assert its use.
type wired struct {
	op       *Operation
	info     *mockprojectInfoAssembler
	specA    *mockspecAssembler
	checker  *mockspecChecker
	renderer *mockreferenceRender
}

func wire(t *testing.T) wired {
	t.Helper()
	w := wired{
		info:     newMockprojectInfoAssembler(t),
		specA:    newMockspecAssembler(t),
		checker:  newMockspecChecker(t),
		renderer: newMockreferenceRender(t),
	}
	w.op = NewOperation(w.info, w.specA, w.checker, w.renderer, false)
	return w
}

func expectHealthySpec(info *mockprojectInfoAssembler, specA *mockspecAssembler, spec arch.Spec) {
	info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
	specA.EXPECT().Assemble(mock.Anything).Return(spec, nil)
}

// tests -------------------------------------------------------------------

// An unreadable project must surface as a ConfigError (exit 2), not a
// system error — the documented contract IsConfigError relies on.
func TestBehave_ProjectInfoFailureIsConfigError(t *testing.T) {
	w := wire(t)
	w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
		Return(domain.Project{}, errors.New("go.mod not found"))

	out, err := w.op.Behave(context.Background(), models.CmdCheckIn{})
	assert.Empty(t, out, "no output model on config error")
	require.Error(t, err, "project-info failure must surface")
	assert.True(t, models.IsConfigError(err), "must classify as config error, got %v", err)
}

// A spec the assembler rejects is a system error path — the message must
// carry the cause.
func TestBehave_SpecAssembleFailureWraps(t *testing.T) {
	w := wire(t)
	w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
	w.specA.EXPECT().Assemble(mock.Anything).Return(arch.Spec{}, errors.New("bad glob"))

	_, err := w.op.Behave(context.Background(), models.CmdCheckIn{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "bad glob")
	assert.False(t, models.IsConfigError(err), "assembler failure is a system error")
}

// Clean project + valid spec: nil error, no warnings, qualities table
// reflects the spec's optional rules.
func TestBehave_CleanRun(t *testing.T) {
	w := wire(t)
	expectHealthySpec(w.info, w.specA, testSpec())
	w.checker.EXPECT().Check(mock.Anything, mock.Anything).Return(models.CheckResult{}, nil)

	out, err := w.op.Behave(context.Background(), models.CmdCheckIn{MaxWarnings: 512})
	require.NoError(t, err)
	assert.False(t, out.ArchHasWarnings)
	assert.Equal(t, "example.com/m", out.ModuleName)
	assert.Empty(t, out.ArchWarningsDependency)
	assert.Equal(t, 0, out.OmittedCount)
}

// Violations flip the result to UserSpaceError (exit 1) with the
// warnings attached.
func TestBehave_ViolationsAreUserSpaceError(t *testing.T) {
	w := wire(t)
	expectHealthySpec(w.info, w.specA, testSpec())
	w.checker.EXPECT().Check(mock.Anything, mock.Anything).Return(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("a"), depWarning("b")},
	}, nil)

	out, err := w.op.Behave(context.Background(), models.CmdCheckIn{MaxWarnings: 512})
	require.Error(t, err)
	assert.True(t, models.IsUserSpaceError(err), "violations must map to exit 1, got %v", err)
	assert.True(t, out.ArchHasWarnings)
	assert.Len(t, out.ArchWarningsDependency, 2)
}

// The MaxWarnings cap trims the warning lists and reports the omitted
// count; the suppressed-count metadata survives the cut.
func TestBehave_MaxWarningsCapCountsOmitted(t *testing.T) {
	w := wire(t)
	expectHealthySpec(w.info, w.specA, testSpec())
	w.checker.EXPECT().Check(mock.Anything, mock.Anything).Return(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("a"), depWarning("b"), depWarning("c")},
		SuppressedCount:    7,
	}, nil)

	out, err := w.op.Behave(context.Background(), models.CmdCheckIn{MaxWarnings: 2})
	require.Error(t, err, "still a violation run")
	assert.Len(t, out.ArchWarningsDependency, 2, "cap keeps two")
	assert.Equal(t, 1, out.OmittedCount, "one warning omitted past the cap")
	assert.Equal(t, 7, out.SuppressedCount, "suppression metadata must survive limiting")
}

// Document notices make the SPEC invalid: no checker run happens, the
// error is a config error (exit 2).
func TestBehave_InvalidSpecSkipsChecker(t *testing.T) {
	w := wire(t)
	spec := testSpec()
	spec.Integrity.DocumentNotices = []arch.Notice{
		{Notice: errors.New("at least one component must be defined")},
	}
	expectHealthySpec(w.info, w.specA, spec)
	// No Check expectation on purpose: an EXPECT().Times(0) still counts
	// as an expectation in testify mocks; AssertNotCalled is the idiom.
	w.renderer.EXPECT().SourceCode(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := w.op.Behave(context.Background(), models.CmdCheckIn{MaxWarnings: 512})
	require.Error(t, err)
	assert.True(t, models.IsConfigError(err), "invalid spec must map to exit 2, got %v", err)
	w.checker.AssertNotCalled(t, "Check", mock.Anything, mock.Anything)
}
