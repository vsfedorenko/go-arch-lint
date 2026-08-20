package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * Launcher-dialect translation: the delegated cobra tree must accept the
 * spellings the launcher documents (-p, --no-colors, selfInspect) and a
 * bare invocation must keep defaulting to `check`.
 */

// Shared flag spellings (goconst: 3+ occurrences each).
const (
	tcProjectPathFlag = "--project-path"
	tcTmpPath         = "/tmp/x"
	tcMaxWarnings     = "--max-warnings"
	tcFocus           = "--focus"
)

func TestTranslateLauncherArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "empty defaults to check",
			in:   nil,
			want: []string{cmdCheck},
		},
		{
			name: "flags only default to check",
			in:   []string{flagNoColors},
			want: []string{cmdCheck, "--output-color=false"},
		},
		{
			name: "short project path",
			in:   []string{"-p", tcTmpPath},
			want: []string{cmdCheck, tcProjectPathFlag, tcTmpPath},
		},
		{
			name: "short project path equals form",
			in:   []string{cmdCheck, "-p=" + tcTmpPath},
			want: []string{cmdCheck, tcProjectPathFlag + "=" + tcTmpPath},
		},
		{
			name: "no-colors translated",
			in:   []string{cmdCheck, flagNoColors},
			want: []string{cmdCheck, "--output-color=false"},
		},
		{
			name: "selfInspect camelCase translated",
			in:   []string{"selfInspect", "--json"},
			want: []string{"self-inspect", "--json"},
		},
		{
			name: "command token survives",
			in:   []string{"mapping", tcProjectPathFlag, tcTmpPath},
			want: []string{"mapping", tcProjectPathFlag, tcTmpPath},
		},
		{
			name: "flag values are not mistaken for commands",
			in:   []string{tcMaxWarnings, "10", "check"},
			want: []string{tcMaxWarnings, "10", cmdCheck},
		},
		{
			name: "equals-form flag values consume no token",
			in:   []string{tcMaxWarnings + "=10"},
			want: []string{cmdCheck, tcMaxWarnings + "=10"},
		},
		{
			name: "graph out flag value is not a command",
			in:   []string{"graph", flagOut, "g.svg", tcFocus, "services"},
			want: []string{"graph", flagOut, "g.svg", tcFocus, "services"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TranslateLauncherArgs(tc.in))
		})
	}
}
