package week

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type copyFlags struct {
	profile string
	json    bool
}

type weekDraftCopyResult struct {
	Schema string           `json:"schema"`
	Draft  domain.WeekDraft `json:"draft"`
}

func newCopyCmd() *cobra.Command {
	var f copyFlags
	cmd := &cobra.Command{
		Use:   "copy <src> <dst>",
		Short: "Clone a draft into a new (date, name) ref",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd, f, args[0], args[1])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runCopy(cmd *cobra.Command, f copyFlags, srcRef, dstRef string) error {
	srcWeek, srcName, err := ParseDraftRef(srcRef)
	if err != nil {
		return fmt.Errorf("src: %w", err)
	}
	dstWeek, dstName, err := ParseDraftRef(dstRef)
	if err != nil {
		return fmt.Errorf("dst: %w", err)
	}

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)
	drafts := draftsvc.NewService(paths, tsvc)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	dst, err := drafts.Copy(profileName, srcWeek, srcName, profileName, dstWeek, dstName)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeCopyResultJSON(w, dst)
	}
	_, _ = fmt.Fprintf(w, "Copied draft %s/%s -> %s/%s.\n",
		srcWeek.Format("2006-01-02"), srcName, dstWeek.Format("2006-01-02"), dstName)
	return nil
}

func writeCopyResultJSON(w io.Writer, d domain.WeekDraft) error {
	return json.NewEncoder(w).Encode(weekDraftCopyResult{
		Schema: "tdx.v1.weekDraftCopyResult", Draft: d,
	})
}
