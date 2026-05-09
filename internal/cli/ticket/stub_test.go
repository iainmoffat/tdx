//nolint:unused // all methods populated by Tasks 9-16
package ticket

import (
	"context"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// stubTicketsvc is the shared test stub for ticketsvcAPI. Tests configure
// only the fields/methods they exercise; everything else returns zero values.
//
//nolint:unused // populated by Tasks 9-16
type stubTicketsvc struct {
	apps            []domain.TicketApp
	statuses        []domain.TicketStatus
	types           []domain.TicketType
	resolvedStatus  domain.TicketStatus
	resolvedType    domain.TicketType
	tickets         []domain.Ticket
	ticket          domain.Ticket
	patched         domain.Ticket
	feed            []domain.TicketFeedEntry
	feedAddedID     int
	savedSearches   []domain.TicketSavedSearch
	resolvedSaved   domain.TicketSavedSearch
	savedResults    []domain.Ticket
	groups          []domain.TicketGroup
	resolvedGroup   domain.TicketGroup
	tasks           []domain.TicketTask
	task            domain.TicketTask
	taskFeed        []domain.TicketFeedEntry
	taskFeedAddedID int

	// Capture last-call arguments for assertion in tests.
	lastFilter      domain.TicketSearchFilter
	lastPatchOps    []ticketsvc.PatchOp
	lastFeedBody    string
	lastFeedPrivate bool
	lastFeedNotify  []string
	lastTaskUpdate  struct {
		Body            string
		PercentComplete *int
		HoursWorked     float64
		IsPrivate       bool
		Notify          []string
	}

	// Inject errors for tests that need them.
	err error
}

func (s *stubTicketsvc) ListApps(_ context.Context, _ string) ([]domain.TicketApp, error) {
	return s.apps, s.err
}
func (s *stubTicketsvc) ListStatuses(_ context.Context, _ string, _ int) ([]domain.TicketStatus, error) {
	return s.statuses, s.err
}
func (s *stubTicketsvc) ListTypes(_ context.Context, _ string, _ int) ([]domain.TicketType, error) {
	return s.types, s.err
}
func (s *stubTicketsvc) ResolveStatusByName(_ context.Context, _ string, _ int, _ string) (domain.TicketStatus, error) {
	return s.resolvedStatus, s.err
}
func (s *stubTicketsvc) ResolveTypeByName(_ context.Context, _ string, _ int, _ string) (domain.TicketType, error) {
	return s.resolvedType, s.err
}
func (s *stubTicketsvc) GetTicket(_ context.Context, _ string, _ int, _ int) (domain.Ticket, error) {
	return s.ticket, s.err
}
func (s *stubTicketsvc) SearchTickets(_ context.Context, _ string, filter domain.TicketSearchFilter) ([]domain.Ticket, error) {
	s.lastFilter = filter
	return s.tickets, s.err
}
func (s *stubTicketsvc) PatchTicket(_ context.Context, _ string, _ int, _ int, ops []ticketsvc.PatchOp) (domain.Ticket, error) {
	s.lastPatchOps = ops
	return s.patched, s.err
}
func (s *stubTicketsvc) GetFeed(_ context.Context, _ string, _ int, _ int) ([]domain.TicketFeedEntry, error) {
	return s.feed, s.err
}
func (s *stubTicketsvc) AddFeed(_ context.Context, _ string, _ int, _ int, body string, isPrivate bool, notify []string) (int, error) {
	s.lastFeedBody = body
	s.lastFeedPrivate = isPrivate
	s.lastFeedNotify = notify
	return s.feedAddedID, s.err
}
func (s *stubTicketsvc) ListSavedSearches(_ context.Context, _ string, _ int) ([]domain.TicketSavedSearch, error) {
	return s.savedSearches, s.err
}
func (s *stubTicketsvc) RunSavedSearch(_ context.Context, _ string, _ int, _ int, _ int) ([]domain.Ticket, error) {
	return s.savedResults, s.err
}
func (s *stubTicketsvc) ResolveSavedSearchByName(_ context.Context, _ string, _ int, _ string) (domain.TicketSavedSearch, error) {
	return s.resolvedSaved, s.err
}
func (s *stubTicketsvc) ListGroups(_ context.Context, _ string) ([]domain.TicketGroup, error) {
	return s.groups, s.err
}
func (s *stubTicketsvc) ResolveGroupByName(_ context.Context, _ string, _ string) (domain.TicketGroup, error) {
	return s.resolvedGroup, s.err
}
func (s *stubTicketsvc) ListTasks(_ context.Context, _ string, _, _ int) ([]domain.TicketTask, error) {
	return s.tasks, s.err
}
func (s *stubTicketsvc) GetTask(_ context.Context, _ string, _, _, _ int) (domain.TicketTask, error) {
	return s.task, s.err
}
func (s *stubTicketsvc) GetTaskFeed(_ context.Context, _ string, _, _, _ int) ([]domain.TicketFeedEntry, error) {
	return s.taskFeed, s.err
}
func (s *stubTicketsvc) UpdateTaskFeed(_ context.Context, _ string, _, _, _ int, body string, pc *int, hw float64, isPrivate bool, notify []string) (int, error) {
	s.lastTaskUpdate.Body = body
	s.lastTaskUpdate.PercentComplete = pc
	s.lastTaskUpdate.HoursWorked = hw
	s.lastTaskUpdate.IsPrivate = isPrivate
	s.lastTaskUpdate.Notify = notify
	return s.taskFeedAddedID, s.err
}
