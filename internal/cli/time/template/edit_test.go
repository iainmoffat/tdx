package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditCmd_NotFound(t *testing.T) {
	_ = seedTemplateDir(t)
	cmd := newEditCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
