package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// vendorBaseName must name the library, not the major-version suffix:
// "/v5" bases fall back to the parent segment, plain bases stay, and
// degenerate paths never panic. Whatever segment wins must be a VALID
// Go identifier fragment: hyphens fold away ("go-git" -> "gogit"),
// because the name becomes the scaffolded variable identifier — an
// unsanitized segment emits a spec that does not compile.
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
		{"hyphenated parent of major suffix", "github.com/go-git/go-git/v5", "gogit"},
		{"hyphenated base", "example.com/lib/go-cache", "gocache"},
		{"dotted base", "gopkg.in/yaml.v3", "yamlv3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, vendorBaseName(tt.in))
		})
	}
}

// TestVendorVarNames pins identifier derivation for vendors: keyword
// escapes, digit-leading prefixes ("4geese" is not a valid Go identifier)
// and collision suffixes between sanitized names.
func TestVendorVarNames(t *testing.T) {
	names := vendorVarNames([]string{
		"example.com/lib/4geese",
		"example.com/other/4geese",
		"example.com/x/type",
		"github.com/go-git/go-git/v5",
		"example.com/plain/gogit",
	})
	assert.Equal(t, "v4geese", names["example.com/lib/4geese"], "digit-leading name gets a v prefix")
	assert.Equal(t, "v4geese2", names["example.com/other/4geese"], "collision after prefix gets a numeric suffix")
	assert.Equal(t, "type_", names["example.com/x/type"], "keyword gets a trailing underscore")
	assert.Equal(t, "gogit", names["github.com/go-git/go-git/v5"], "hyphen folds away")
	assert.Equal(t, "gogit2", names["example.com/plain/gogit"], "sanitized name colliding with a plain name renumbers")
}
