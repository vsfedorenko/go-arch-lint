package test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/services/checker/deepscan"
)

// The nested fixture module must stay resolvable: its go.mod module path
// matches the import paths used by its own sources, so the searcher can
// find cross-package implementations. This is the regression pin for a
// long-standing rot where the module path drifted from the import paths
// (leftover from the upstream layout shuffles) and every implementation
// lookup silently returned zero results.
func TestFixtureModuleResolvable(t *testing.T) {
	_, callerDir, _, _ := runtime.Caller(0)
	projectDir := filepath.Join(filepath.Dir(callerDir), "project")

	searcher := deepscan.NewSearcher()
	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(filepath.Join(projectDir, "internal", "operations")),
		deepscan.WithAnalyseScope(filepath.Join(projectDir, "internal")),
	)
	require.NoError(t, err)

	methods, err := searcher.Usages(criteria)
	require.NoError(t, err)

	found := false
	for _, method := range methods {
		if method.Name != "NewProcessorBasic1" {
			continue
		}

		require.Len(t, method.Gates, 1)
		gate := method.Gates[0]
		require.Len(t, gate.Implementations, 1)
		assert.Equal(t, "Memory", gate.Implementations[0].Target.StructName)
		found = true
	}

	assert.True(t, found, "NewProcessorBasic1 must be discovered with its repository.Memory injection")
}

func TestIssue85MultiReturnTuple(t *testing.T) {
	// Regression for issue #85: a multi-value return spread as a single
	// argument tuple (foo(bar())) gives callExpr.Args fewer entries than the
	// callee has parameters, which used to panic the deep-scan argument walk.
	_, callerDir, _, _ := runtime.Caller(0)
	projectDir := filepath.Join(filepath.Dir(callerDir), "project_issue85")

	searcher := deepscan.NewSearcher()
	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(filepath.Join(projectDir, "internal")),
	)
	require.NoError(t, err)

	_, err = searcher.Usages(criteria)
	assert.NoError(t, err)
}
