package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newAssignCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		commentFlag string
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "assign <id> <uid|email|me>",
		Short: "Reassign a ticket (--yes required)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to reassign")
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
			people := peoplesvc.New(paths)
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}
			uid, err := resolvePrincipal(cmd.Context(), people, profile, authedUID, args[1])
			if err != nil {
				return err
			}
			return runTicketAssign(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, id, uid, commentFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "optional accompanying feed comment")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketAssign(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id int, newUID, comment string) error {
	updated, err := svc.PatchTicket(ctx, profile, appID, id, []ticketsvc.PatchOp{
		{Op: "replace", Path: "/ResponsibleUid", Value: newUID},
	})
	if err != nil {
		return err
	}
	name := updated.ResponsibleName
	if name == "" {
		name = newUID
	}
	_, _ = fmt.Fprintf(w, "ticket #%d reassigned → %s\n", id, name)
	if comment != "" {
		if _, ferr := svc.AddFeed(ctx, profile, appID, id, comment, false, nil); ferr != nil {
			_, _ = fmt.Fprintf(w, "warning: reassignment succeeded but comment failed: %v\n", ferr)
		}
	}
	return nil
}
