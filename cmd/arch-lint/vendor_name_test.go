package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// vendorBaseName must name the library, not the major-version suffix:
// "/v5" bases fall back to the parent segment, plain bases stay, and
// degenerate paths never panic.
func TestVendorBaseName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"major suffix", "github.com/go-chi/chi/v5", "chi"},
		{"major suffix v2", "example.com/lib/v2", "lib"},
		{"plain base", "golang.org/x/text/cases", "cases"},
		{"no version", "github.com/stretchr/testify", "testify"},
		{"single element", "single", "single"},
		{"weird suffix", "example.com/lib/vX", "vx"}, // not numeric: stays (sanitized)
		{"v1 suffix", "example.com/lib/v1", "v1"},    // v1 is not /vN>1: stays
		{"root-ish", "/v3", "v3"},                    // no parent: stays
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, vendorBaseName(tt.in))
		})
	}
}
