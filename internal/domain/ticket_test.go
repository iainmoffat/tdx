package domain

import "testing"

func TestTicketIsFullDefault(t *testing.T) {
	tk := Ticket{ID: 1}
	if tk.IsFull {
		t.Fatal("Ticket.IsFull must default to false")
	}
}

func TestTicketSearchFilterZeroValueIsValid(t *testing.T) {
	// No fields are required at construction time — service layer applies defaults.
	_ = TicketSearchFilter{}
}

func TestTicketGroupZeroValueIsValid(t *testing.T) {
	_ = TicketGroup{}
}

func TestTicketSearchFilterResponsibilityGroupIDs(t *testing.T) {
	f := TicketSearchFilter{ResponsibilityGroupIDs: []int{1, 2}}
	if len(f.ResponsibilityGroupIDs) != 2 {
		t.Fatalf("want 2, got %d", len(f.ResponsibilityGroupIDs))
	}
}

func TestTicketTaskZeroValueIsValid(t *testing.T) {
	_ = TicketTask{}
}

func TestTicketTaskFields(t *testing.T) {
	tk := TicketTask{ID: 1, TicketID: 100, Title: "x", PercentComplete: 50}
	if tk.PercentComplete != 50 {
		t.Fatalf("PercentComplete: got %d, want 50", tk.PercentComplete)
	}
}
