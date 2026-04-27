package domain

import (
	"testing"
)

func TestActionKind_StringIncludesDelete(t *testing.T) {
	if got, want := ActionDelete.String(), "delete"; got != want {
		t.Errorf("ActionDelete.String() = %q, want %q", got, want)
	}
}

func TestReconcileDiff_CountByKindIncludesDelete(t *testing.T) {
	diff := ReconcileDiff{Actions: []Action{
		{Kind: ActionCreate}, {Kind: ActionUpdate}, {Kind: ActionDelete},
		{Kind: ActionDelete}, {Kind: ActionSkip},
	}}
	creates, updates, deletes, skips := diff.CountByKindV2()
	if creates != 1 || updates != 1 || deletes != 2 || skips != 1 {
		t.Errorf("counts wrong: %d/%d/%d/%d", creates, updates, deletes, skips)
	}
}
