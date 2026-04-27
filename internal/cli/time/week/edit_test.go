package week

import (
	"testing"

	"github.com/spf13/cobra"
)

// Note: full edit-loop tests would require mocking $EDITOR. Phase A relies
// on manual walkthrough for the editor path. Here we just verify the command
// constructs cleanly with the expected flags.
func TestEditCmd_ConstructsCleanly(t *testing.T) {
	cmd := newEditCmd()
	if cmd.Use == "" {
		t.Errorf("command Use is empty")
	}
	if got := cmd.Flags().Lookup("profile"); got == nil {
		t.Errorf("--profile flag missing")
	}
}

var _ = cobra.Command{}
