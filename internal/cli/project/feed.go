package project

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

func newFeedCmd(svc projectsvcAPI) *cobra.Command {
	var (
		limit       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "feed <project-id>",
		Short: "Read the feed for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
			}
			if limit < 0 {
				limit = 0
			}
			if limit > 500 {
				limit = 500
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
			return runProjectFeed(cmd.Context(), cmd.OutOrStdout(), s, profile, id, limit, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max entries (0 = all, max 500)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runProjectFeed(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, projectID, limit int, jsonOut bool) error {
	entries, err := svc.GetFeed(ctx, profile, projectID)
	if err != nil {
		return err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return printProjectFeed(w, entries, jsonOut, "tdx.v1.projectFeed", map[string]any{
		"projectID": projectID,
	})
}

func printProjectFeed(w io.Writer, entries []domain.ProjectFeedEntry, jsonOut bool, schemaTag string, extras map[string]any) error {
	if jsonOut {
		return printProjectFeedJSON(w, entries, schemaTag, extras)
	}
	return printProjectFeedText(w, entries, extras)
}

func printProjectFeedText(w io.Writer, entries []domain.ProjectFeedEntry, extras map[string]any) error {
	if len(entries) == 0 {
		projectID, _ := extras["projectID"].(int)
		if taskID, ok := extras["taskID"].(int); ok && taskID > 0 {
			_, _ = fmt.Fprintf(w, "no feed entries on task #%d\n", taskID)
		} else {
			_, _ = fmt.Fprintf(w, "no feed entries on project %d\n", projectID)
		}
		return nil
	}
	headers := []string{"ID", "DATE", "BY", "TYPE", "BODY"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		when := ""
		if !e.CreatedDate.IsZero() {
			when = e.CreatedDate.Format("2006-01-02 15:04")
		}
		body := strings.ReplaceAll(e.Body, "\n", "↵")
		body = truncate(body, 80)
		rows = append(rows, []string{
			strconv.Itoa(e.ID),
			when,
			truncate(e.CreatedByName, 20),
			e.UpdateTypeLabel(),
			body,
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

func printProjectFeedJSON(w io.Writer, entries []domain.ProjectFeedEntry, schemaTag string, extras map[string]any) error {
	type entryJSON struct {
		ID            int    `json:"id"`
		CreatedByUID  string `json:"createdByUID,omitempty"`
		CreatedByName string `json:"createdByName,omitempty"`
		CreatedDate   string `json:"createdDate,omitempty"`
		UpdateType    string `json:"updateType,omitempty"`
		Body          string `json:"body"`
		IsPrivate     bool   `json:"isPrivate"`
		LikesCount    int    `json:"likesCount,omitempty"`
		RepliesCount  int    `json:"repliesCount,omitempty"`
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		ts := ""
		if !e.CreatedDate.IsZero() {
			ts = e.CreatedDate.Format(time.RFC3339)
		}
		out = append(out, entryJSON{
			ID:            e.ID,
			CreatedByUID:  e.CreatedByUID,
			CreatedByName: e.CreatedByName,
			CreatedDate:   ts,
			UpdateType:    e.UpdateTypeLabel(),
			Body:          e.Body,
			IsPrivate:     e.IsPrivate,
			LikesCount:    e.LikesCount,
			RepliesCount:  e.RepliesCount,
		})
	}
	type envelope struct {
		Schema    string      `json:"schema"`
		ProjectID int         `json:"projectID,omitempty"`
		PlanID    int         `json:"planID,omitempty"`
		TaskID    int         `json:"taskID,omitempty"`
		Entries   []entryJSON `json:"entries"`
	}
	env := envelope{Schema: schemaTag, Entries: out}
	if v, ok := extras["projectID"].(int); ok {
		env.ProjectID = v
	}
	if v, ok := extras["planID"].(int); ok {
		env.PlanID = v
	}
	if v, ok := extras["taskID"].(int); ok {
		env.TaskID = v
	}
	return render.JSON(w, env)
}
