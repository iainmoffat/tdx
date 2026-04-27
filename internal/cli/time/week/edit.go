package week

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type editFlags struct {
	profile string
}

func newEditCmd() *cobra.Command {
	var f editFlags
	cmd := &cobra.Command{
		Use:   "edit <date>[/<name>]",
		Short: "Edit a draft as YAML in $EDITOR",
		Long: `Open the draft's YAML file in $EDITOR (defaults to vi) for in-place editing.

Phase A MVP uses YAML-text editing. A grid-aware TUI editor for drafts is
planned for a later phase. Saved YAML is validated against the WeekDraft
schema before being written back to disk; an invalid edit is rejected.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return cmd
}

func runEdit(cmd *cobra.Command, f editFlags, ref string) error {
	weekStart, name, err := ParseDraftRef(ref)
	if err != nil {
		return err
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

	d, err := drafts.Store().Load(profileName, weekStart, name)
	if err != nil {
		return err
	}

	initial, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal draft: %w", err)
	}

	edited, err := openYAMLEditor(string(initial))
	if err != nil {
		return err
	}

	var updated domain.WeekDraft
	if err := yaml.Unmarshal([]byte(edited), &updated); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	if err := updated.Validate(); err != nil {
		return fmt.Errorf("invalid draft: %w", err)
	}
	// Preserve identity fields against accidental edits.
	if updated.Profile != d.Profile {
		return fmt.Errorf("profile cannot be changed via edit (%q -> %q)", d.Profile, updated.Profile)
	}
	if !updated.WeekStart.Equal(d.WeekStart) {
		return fmt.Errorf("weekStart cannot be changed via edit")
	}
	if updated.Name != d.Name {
		return fmt.Errorf("name cannot be changed via edit (use copy/rename in a future phase)")
	}
	updated.ModifiedAt = time.Now().UTC()

	if err := drafts.Store().Save(updated); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved draft %s/%s.\n",
		weekStart.Format("2006-01-02"), name)
	return nil
}

// openYAMLEditor writes initial to a temp file, invokes $EDITOR (vi fallback),
// and returns the user-edited contents.
func openYAMLEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	file, err := os.CreateTemp("", "tdx-week-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(initial); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	_ = file.Close()
	defer func() { _ = os.Remove(file.Name()) }()

	c := exec.Command(editor, file.Name())
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", editor, err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}
	return string(data), nil
}
