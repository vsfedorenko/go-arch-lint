package checker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
)

/**
 * InterfacePlacement checker tests: real .go files on disk, real parser,
 * synthetic project trees. No network, no type loading — the checker is
 * syntax-only by design.
 */

// singleConsumerProject: interface in beta, consumed only by alpha —
// the violation case.
func singleConsumerProject(t *testing.T) (arch.Spec, []models.FileHold) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "internal/beta/iface.go", "package beta\n\ntype Iface interface {\n\tDo() error\n}\n")
	writeFile(t, root, "internal/beta/impl.go", "package beta\n\ntype Impl struct{}\n\nfunc (Impl) Do() error { return nil }\n")
	writeFile(t, root, "internal/alpha/use.go", "package alpha\n\nimport \"example.com/proj/internal/beta\"\n\nfunc Use(x beta.Iface) error { return x.Do() }\n")

	alpha, beta := "alpha", "beta"
	return ifSpecFor(root), holdsFor(t, root, map[string]*string{
		"internal/beta/iface.go": &beta,
		"internal/beta/impl.go":  &beta,
		"internal/alpha/use.go":  &alpha,
	})
}

func TestInterfacePlacement_single_consumer_in_other_component(t *testing.T) {
	spec, holds := singleConsumerProject(t)

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), spec)
	require.NoError(t, err)

	require.Len(t, result.DependencyWarnings, 1)
	w := result.DependencyWarnings[0]
	assert.Contains(t, w.ComponentName, "interface 'Iface' must live with its consumer 'alpha'")
	assert.Contains(t, w.ComponentName, "declared in component 'beta'")
	assert.Contains(t, w.FileRelativePath, "iface.go")
}

func TestInterfacePlacement_no_rule_noop(t *testing.T) {
	spec, holds := singleConsumerProject(t)
	spec.InterfacePlacement = nil

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestInterfacePlacement_same_component_usage_ok(t *testing.T) {
	// Interface declared AND used inside beta only — no cross-component
	// consumer, no violation.
	root := t.TempDir()
	writeFile(t, root, "internal/beta/iface.go", "package beta\n\ntype Local interface{ Do() error }\n")
	writeFile(t, root, "internal/beta/use.go", "package beta\n\nfunc Use(x Local) error { return x.Do() }\n")
	writeFile(t, root, "internal/alpha/alpha.go", "package alpha\n")

	alpha, beta := "alpha", "beta"
	holds := holdsFor(t, root, map[string]*string{
		"internal/beta/iface.go":  &beta,
		"internal/beta/use.go":    &beta,
		"internal/alpha/alpha.go": &alpha,
	})

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), ifSpecFor(root))
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestInterfacePlacement_shared_by_two_components_ok(t *testing.T) {
	// Iface consumed by alpha AND gamma — genuinely shared, may stay.
	root := t.TempDir()
	writeFile(t, root, "internal/beta/iface.go", "package beta\n\ntype Iface interface{ Do() error }\n")
	writeFile(t, root, "internal/alpha/use.go", "package alpha\n\nimport \"example.com/proj/internal/beta\"\n\nfunc A(x beta.Iface) error { return x.Do() }\n")
	writeFile(t, root, "internal/gamma/use.go", "package gamma\n\nimport \"example.com/proj/internal/beta\"\n\nfunc G(x beta.Iface) error { return x.Do() }\n")

	alpha, beta, gamma := "alpha", "beta", "gamma"
	holds := holdsFor(t, root, map[string]*string{
		"internal/beta/iface.go": &beta,
		"internal/alpha/use.go":  &alpha,
		"internal/gamma/use.go":  &gamma,
	})

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), ifSpecFor(root))
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestInterfacePlacement_two_files_same_consumer_one_violation(t *testing.T) {
	// Two files of the SAME consuming component — one consumer component,
	// still a single violation.
	root := t.TempDir()
	writeFile(t, root, "internal/beta/iface.go", "package beta\n\ntype Iface interface{ Do() error }\n")
	writeFile(t, root, "internal/alpha/use1.go", "package alpha\n\nimport \"example.com/proj/internal/beta\"\n\nfunc A1(x beta.Iface) error { return x.Do() }\n")
	writeFile(t, root, "internal/alpha/use2.go", "package alpha\n\nimport \"example.com/proj/internal/beta\"\n\nfunc A2(x beta.Iface) error { return x.Do() }\n")

	alpha, beta := "alpha", "beta"
	holds := holdsFor(t, root, map[string]*string{
		"internal/beta/iface.go": &beta,
		"internal/alpha/use1.go": &alpha,
		"internal/alpha/use2.go": &alpha,
	})

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), ifSpecFor(root))
	require.NoError(t, err)
	assert.Len(t, result.DependencyWarnings, 1)
}

func TestInterfacePlacement_struct_types_ignored(t *testing.T) {
	// beta.Struct is not an interface — no placement rule applies.
	root := t.TempDir()
	writeFile(t, root, "internal/beta/types.go", "package beta\n\ntype Config struct{ N int }\n")
	writeFile(t, root, "internal/alpha/use.go", "package alpha\n\nimport \"example.com/proj/internal/beta\"\n\nfunc A(c beta.Config) int { return c.N }\n")

	alpha, beta := "alpha", "beta"
	holds := holdsFor(t, root, map[string]*string{
		"internal/beta/types.go": &beta,
		"internal/alpha/use.go":  &alpha,
	})

	result, err := NewInterfacePlacement(fakeProjectFilesResolver{holds: holds}).Check(context.Background(), ifSpecFor(root))
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

// --- helpers ---------------------------------------------------------------

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func ifSpecFor(root string) arch.Spec {
	return arch.Spec{
		RootDirectory:      common.NewReferable(root, common.NewEmptyReference()),
		ModuleName:         common.NewReferable("example.com/proj", common.NewEmptyReference()),
		InterfacePlacement: &arch.InterfacePlacement{MustLiveWithConsumer: true},
	}
}

func holdsFor(t *testing.T, root string, owners map[string]*string) []models.FileHold {
	t.Helper()
	holds := []models.FileHold{}
	for rel, owner := range owners {
		holds = append(holds, models.FileHold{
			File: models.ProjectFile{
				Path:        filepath.Join(root, rel),
				PackageName: filepath.Base(filepath.Dir(rel)),
			},
			ComponentID: owner,
		})
	}
	return holds
}
