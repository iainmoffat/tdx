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

func newCommentCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		isPrivate   bool
		notify      []string
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "comment <id> <message>",
		Short: "Post a feed comment to a ticket (--yes required)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to post the comment")
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
			return runTicketComment(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, id, args[1], isPrivate, notify)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&isPrivate, "private", false, "internal note (not visible to requestor)")
	cmd.Flags().StringSliceVar(&notify, "notify", nil, "additional notify recipients by UID (repeatable)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to post")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketComment(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id int, body string, isPrivate bool, notify []string) error {
	feedID, err := svc.AddFeed(ctx, profile, appID, id, body, isPrivate, notify)
	if err != nil {
		return err
	}
	visibility := "public"
	if isPrivate {
		visibility = "private"
	}
	_, _ = fmt.Fprintf(w, "comment posted to ticket #%d (feed entry %d, %s)\n", id, feedID, visibility)
	return nil
}
