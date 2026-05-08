package ticket

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// ticketsvcAPI is the minimal interface CLI commands depend on.
// Defined here (not in the service package) so tests can stub easily.
//
//nolint:unused // consumed by Tasks 9-16 subcommands
type ticketsvcAPI interface {
	ListApps(ctx context.Context, profile string) ([]domain.TicketApp, error)
	ListStatuses(ctx context.Context, profile string, appID int) ([]domain.TicketStatus, error)
	ListTypes(ctx context.Context, profile string, appID int) ([]domain.TicketType, error)
	ResolveStatusByName(ctx context.Context, profile string, appID int, name string) (domain.TicketStatus, error)
	GetTicket(ctx context.Context, profile string, appID, id int) (domain.Ticket, error)
	SearchTickets(ctx context.Context, profile string, filter domain.TicketSearchFilter) ([]domain.Ticket, error)
	PatchTicket(ctx context.Context, profile string, appID, id int, ops []ticketsvc.PatchOp) (domain.Ticket, error)
	GetFeed(ctx context.Context, profile string, appID, ticketID int) ([]domain.TicketFeedEntry, error)
	AddFeed(ctx context.Context, profile string, appID, ticketID int, body string, isPrivate bool, notify []string) (int, error)
	ListSavedSearches(ctx context.Context, profile string, appID int) ([]domain.TicketSavedSearch, error)
	RunSavedSearch(ctx context.Context, profile string, appID, searchID, limit int) ([]domain.Ticket, error)
	ResolveSavedSearchByName(ctx context.Context, profile string, appID int, name string) (domain.TicketSavedSearch, error)
}

// New returns the top-level `tdx ticket` command. Subcommands are registered
// in their respective files as they land.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage TeamDynamix tickets",
	}
	cmd.AddCommand(newAppCmd(nil))
	cmd.AddCommand(newTypesCmd(nil))
	cmd.AddCommand(newStatusesCmd(nil))
	return cmd
}
