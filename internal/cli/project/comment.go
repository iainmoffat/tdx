package project

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

func newCommentCmd(svc projectsvcAPI) *cobra.Command {
	var (
		message     string
		notify      []string
		isPrivate   bool
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "comment <project-id>",
		Short: "Post a comment to a project feed (--yes required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate BEFORE config.ResolvePaths so unit tests without
			// a real config on disk still hit the right errors.
			if !yesFlag {
				return fmt.Errorf("pass --yes to post comment")
			}
			if strings.TrimSpace(message) == "" {
				return fmt.Errorf("specify --message")
			}
			projectID, err := strconv.Atoi(args[0])
			if err != nil || projectID <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
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
				s = projectsvc.New(paths)
			}

			// Resolve --notify values.
			var resolvedNotify []string
			if len(notify) > 0 {
				var authedUID string
				for _, n := range notify {
					if n == "me" && authedUID == "" {
						authedUID, err = authedUIDFor(cmd.Context(), auth, profile)
						if err != nil {
							return err
						}
					}
					uid, err := resolvePrincipal(cmd.Context(), peoplesvc.New(paths), profile, authedUID, n)
					if err != nil {
						return fmt.Errorf("resolve notify %q: %w", n, err)
					}
					resolvedNotify = append(resolvedNotify, uid)
				}
			}

			return runProjectComment(cmd.Context(), cmd.OutOrStdout(), s, profile, projectID, message, isPrivate, resolvedNotify)
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "comment text (required)")
	cmd.Flags().StringArrayVar(&notify, "notify", nil, "additional UID to notify (repeatable; 'me' = authed user)")
	cmd.Flags().BoolVar(&isPrivate, "private", false, "mark as private/internal note")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to post")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runProjectComment(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, projectID int, message string, isPrivate bool, notify []string) error {
	entryID, err := svc.AddFeed(ctx, profile, projectID, message, isPrivate, notify)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "posted comment on project %d (feed entry %d)\n", projectID, entryID)
	return nil
}
