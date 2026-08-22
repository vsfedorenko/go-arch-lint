package archlint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunEmptySpecDefReturnsError(t *testing.T) {
	err := Run(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}
