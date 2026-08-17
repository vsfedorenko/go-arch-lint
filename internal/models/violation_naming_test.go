package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToViolations_Naming(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsNaming: []CheckArchWarningNaming{
			{PackageName: "utils", PackagePath: "/internal/utils", FileRelativePath: "/internal/utils/a.go", FilesCount: 3},
		},
	}

	violations := out.ToViolations()
	assert.Len(t, violations, 1)
	assert.Equal(t, "naming", violations[0].Type)
	assert.Equal(t, "/internal/utils/a.go", violations[0].File)
	assert.Equal(t, "utils", violations[0].Package)
	assert.Contains(t, violations[0].Rule, `"utils" is forbidden`)
	assert.Contains(t, violations[0].Rule, "3 file(s)")
}
