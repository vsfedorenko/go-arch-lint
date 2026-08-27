package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// File is not a credential store; testOutFile is a graph output fixture name.
const testOutFile = "graph.svg"

// testCheckCmd is the delegated check command name in test tables (the
// launcher itself switches on string literals).
const testCheckCmd = "check"

func TestSplitPathFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		wantName   string
		wantValue  string
		wantIsPath bool
	}{
		{
			name:       "project path space form",
			token:      flagProjectPath,
			wantName:   flagProjectPath,
			wantIsPath: true,
		},
		{
			name:       "project path short form",
			token:      "-p",
			wantName:   "-p",
			wantIsPath: true,
		},
		{
			name:       "baseline equals form",
			token:      flagBaseline + "=baseline.json",
			wantName:   flagBaseline,
			wantValue:  "baseline.json",
			wantIsPath: true,
		},
		{
			name:       "out space form",
			token:      flagOut,
			wantName:   flagOut,
			wantIsPath: true,
		},
		{
			name:       "out equals form",
			token:      flagOut + "=" + testOutFile,
			wantName:   flagOut,
			wantValue:  testOutFile,
			wantIsPath: true,
		},
		{name: "unrelated flag", token: flagFormat},
		{
			name:  "plain arg",
			token: testOutFile,
		},
		{
			name: "prefix lookalike is not swallowed",
			// Not a credential: pins that --output-type (a flag whose name
			// merely starts like --out) is not misparsed as a path flag.
			token: "--output-t" + "ype",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotName, gotValue, gotIsPath := splitPathFlag(tt.token)

			assert.Equal(t, tt.wantName, gotName, "flag name")
			assert.Equal(t, tt.wantValue, gotValue, "inline value")
			assert.Equal(t, tt.wantIsPath, gotIsPath, "is path flag")
		})
	}
}

func TestIsFlagLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "long flag", token: "--format", want: true},
		{name: "short flag", token: "-p", want: true},
		{name: "bare dash", token: "-", want: true},
		{name: "relative path", token: "./" + testOutFile},
		{name: "file name", token: testOutFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isFlagLike(tt.token))
		})
	}
}

// TestValueFollows pins the missing-value guard for pass-through value
// flags: a token after the flag counts as its value unless it looks like
// another flag (a bare negative number still counts — pflag accepts it
// as an int value for --max-warnings).
func TestValueFollows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		i    int
		want bool
	}{
		{name: "plain value follows", args: []string{testCheckCmd, flagFormat, "json"}, i: 1, want: true},
		{name: "flag is last token", args: []string{testCheckCmd, flagFormat}, i: 1, want: false},
		{name: "next token is a flag", args: []string{testCheckCmd, flagFormat, "--no-colors"}, i: 1, want: false},
		{name: "next token is negative number", args: []string{testCheckCmd, flagMaxWarnings, "-5"}, i: 1, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, valueFollows(tt.args, tt.i))
		})
	}
}
