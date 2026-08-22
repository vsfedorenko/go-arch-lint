package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// File is not a credential store; testOutFile is a graph output fixture name.
const testOutFile = "graph.svg"

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
		{
			name:  "unrelated flag",
			token: "--format",
		},
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
