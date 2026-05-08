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
