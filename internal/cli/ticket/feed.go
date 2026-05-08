package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newFeedCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		limit       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "feed <id>",
		Short: "Read the feed for a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
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
			return runTicketFeed(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, id, limit, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().IntVar(&limit, "limit", 0, "max entries (0 = all)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketFeed(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id, limit int, jsonOut bool) error {
	entries, err := svc.GetFeed(ctx, profile, appID, id)
	if err != nil {
		return err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	if jsonOut {
		return printFeedJSON(w, entries)
	}
	return printFeedText(w, entries)
}

func printFeedText(w io.Writer, entries []domain.TicketFeedEntry) error {
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(w, "no feed entries")
		return nil
	}
	for i, e := range entries {
		when := ""
		if !e.CreatedAt.IsZero() {
			when = e.CreatedAt.Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(w, "[%s] %s — %s\n", when, e.AuthorName, e.EventKind)
		body := e.Body
		if body != "" {
			for _, line := range splitLines(body) {
				_, _ = fmt.Fprintln(w, "  "+line)
			}
		}
		if i < len(entries)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
	return nil
}

func printFeedJSON(w io.Writer, entries []domain.TicketFeedEntry) error {
	type entryJSON struct {
		ID         int    `json:"id"`
		AuthorUID  string `json:"authorUID,omitempty"`
		AuthorName string `json:"authorName,omitempty"`
		CreatedAt  string `json:"createdAt,omitempty"`
		Body       string `json:"body,omitempty"`
		IsPrivate  bool   `json:"isPrivate"`
		EventKind  string `json:"eventKind,omitempty"`
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		ts := ""
		if !e.CreatedAt.IsZero() {
			ts = e.CreatedAt.Format(time.RFC3339)
		}
		out = append(out, entryJSON{
			ID: e.ID, AuthorUID: e.AuthorUID, AuthorName: e.AuthorName,
			CreatedAt: ts, Body: e.Body, IsPrivate: e.IsPrivate, EventKind: e.EventKind,
		})
	}
	return render.JSON(w, struct {
		Schema  string      `json:"schema"`
		Entries []entryJSON `json:"entries"`
	}{Schema: "tdx.v1.ticketFeed", Entries: out})
}
