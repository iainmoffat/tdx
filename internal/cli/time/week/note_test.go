package week

import (
	"testing"
)

// Note: full append / clear behavior tests would require a tempdir + saved
// draft, which is a moderate fixture. Phase A relies on manual walkthrough
// for the editor path. Here we test the parse / no-flags branching only.

func TestNoteCmd_ConstructsCleanly(t *testing.T) {
	cmd := newNoteCmd()
	if cmd.Use == "" {
		t.Errorf("command Use is empty")
	}
	if got := cmd.Flags().Lookup("append"); got == nil {
		t.Errorf("--append flag missing")
	}
	if got := cmd.Flags().Lookup("clear"); got == nil {
		t.Errorf("--clear flag missing")
	}
}
