package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newStatusCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		statusID    int
		commentFlag string
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "status <id> <name-or-id>",
		Short: "Change a ticket's status (--yes required)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to change status")
			}
			parsedID, name := parseStatusArg(args[1])
			if statusID > 0 {
				parsedID = statusID
				name = ""
			}
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			return runTicketStatus(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, id, parsedID, name, commentFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().IntVar(&statusID, "status-id", 0, "status id (overrides positional name)")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "optional accompanying feed comment")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketStatus(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id, statusID int, statusName, comment string) error {
	if statusID == 0 {
		st, err := svc.ResolveStatusByName(ctx, profile, appID, statusName)
		if err != nil {
			return err
		}
		statusID = st.ID
	}
	updated, err := svc.PatchTicket(ctx, profile, appID, id, []ticketsvc.PatchOp{
		{Op: "replace", Path: "/StatusID", Value: statusID},
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "ticket #%d status → %s\n", id, updated.StatusName)
	if comment != "" {
		if _, ferr := svc.AddFeed(ctx, profile, appID, id, comment, false, nil); ferr != nil {
			_, _ = fmt.Fprintf(w, "warning: status changed but comment failed: %v\n", ferr)
		}
	}
	return nil
}
