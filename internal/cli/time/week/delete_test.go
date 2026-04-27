package week

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestDelete_RequiresYes(t *testing.T) {
	cmd := newDeleteCmd()
	cmd.SetArgs([]string{"2026-05-04"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error without --yes")
	}
}

// Reference cobra to avoid unused import if other tests are simplified later.
var _ = cobra.Command{}
