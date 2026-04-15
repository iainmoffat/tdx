package entry

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// ErrPartialDelete is returned when a batch delete partially succeeds.
// Callers can type-assert to check for exit code 2.
type ErrPartialDelete struct {
	Succeeded int
	Failed    int
	Message   string
}

func (e *ErrPartialDelete) Error() string { return e.Message }

type deleteFlags struct {
	profile string
	dryRun  bool
	json    bool
}

type entryDeleteJSON struct {
	Schema  string               `json:"schema"`
	Deleted []int                `json:"deleted"`
	Failed  []entryDeleteFailure `json:"failed,omitempty"`
}

type entryDeleteFailure struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

func newDeleteCmd() *cobra.Command {
	var f deleteFlags

	cmd := &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more time entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]int, len(args))
			for i, arg := range args {
				id, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid entry ID %q", arg)
				}
				ids[i] = id
			}
			return runDelete(cmd, ids, f)
		},
	}

	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "preview entries without deleting them")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit JSON output")

	return cmd
}

func runDelete(cmd *cobra.Command, ids []int, f deleteFlags) error {
	// ---- 1. Resolve profile ----

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	ctx := cmd.Context()

	// ---- 2. Dry run: fetch and display, no deletion ----

	if f.dryRun {
		for _, id := range ids {
			entry, err := tsvc.GetEntry(ctx, profileName, id)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(w, "dry run: would delete entry %d\n", id)
			printEntry(w, entry)
		}
		return nil
	}

	format := render.ResolveFormat(render.Flags{JSON: f.json})

	// ---- 3. Single-ID path ----

	if len(ids) == 1 {
		id := ids[0]
		if err := tsvc.DeleteEntry(ctx, profileName, id); err != nil {
			return err
		}

		if format == render.FormatJSON {
			return render.JSON(w, entryDeleteJSON{
				Schema:  "tdx.v1.entryDelete",
				Deleted: []int{id},
			})
		}

		_, _ = fmt.Fprintf(w, "deleted entry %d\n", id)
		return nil
	}

	// ---- 4. Multi-ID path ----

	result, err := tsvc.DeleteEntries(ctx, profileName, ids)
	if err != nil {
		return err
	}

	if format == render.FormatJSON {
		failures := make([]entryDeleteFailure, len(result.Failed))
		for i, bf := range result.Failed {
			failures[i] = entryDeleteFailure{ID: bf.ID, Message: bf.Message}
		}
		return render.JSON(w, entryDeleteJSON{
			Schema:  "tdx.v1.entryDelete",
			Deleted: result.Succeeded,
			Failed:  failures,
		})
	}

	switch {
	case result.FullSuccess():
		_, _ = fmt.Fprintf(w, "deleted %d entries\n", len(result.Succeeded))

	case result.PartialSuccess():
		_, _ = fmt.Fprintf(w, "deleted %d entries\n", len(result.Succeeded))
		_, _ = fmt.Fprintf(w, "failed to delete %d entries:\n", len(result.Failed))
		for _, bf := range result.Failed {
			_, _ = fmt.Fprintf(w, "  entry %d: %s\n", bf.ID, bf.Message)
		}
		return &ErrPartialDelete{
			Succeeded: len(result.Succeeded),
			Failed:    len(result.Failed),
			Message: fmt.Sprintf("partial delete: %d succeeded, %d failed",
				len(result.Succeeded), len(result.Failed)),
		}

	case result.TotalFailure():
		return fmt.Errorf("all %d deletes failed", len(result.Failed))
	}

	return nil
}
