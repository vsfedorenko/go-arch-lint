package view

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// templates_test.go guards the embedded-template map: every command
// output model must have exactly one non-empty template — a missing one
// would blank the HTML view for that command, an extra one is dead weight.

func TestTemplatesCoverAllModels(t *testing.T) {
	require.Len(t, Templates, 6)
	for _, m := range []any{
		models.CmdCheckOut{},
		models.CmdErrorOut{},
		models.CmdGraphOut{},
		models.CmdMappingOut{},
		models.CmdSelfInspectOut{},
		models.CmdVersionOut{},
	} {
		tpl, ok := Templates[fmt.Sprintf("%T", m)]
		require.True(t, ok, "no template registered for %T", m)
		assert.NotEmpty(t, tpl, "empty template for %T", m)
	}
}
