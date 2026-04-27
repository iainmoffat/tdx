package week

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type noteFlags struct {
	profile    string
	appendText string
	clear      bool
}

func newNoteCmd() *cobra.Command {
	var f noteFlags
	cmd := &cobra.Command{
		Use:   "note <date>[/<name>]",
		Short: "Edit free-form notes on a draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNote(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.appendText, "append", "", "append text without invoking $EDITOR")
	cmd.Flags().BoolVar(&f.clear, "clear", false, "clear notes")
	return cmd
}

func runNote(cmd *cobra.Command, f noteFlags, ref string) error {
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

	switch {
	case f.clear:
		d.Notes = ""
	case f.appendText != "":
		if d.Notes != "" && !strings.HasSuffix(d.Notes, "\n") {
			d.Notes += "\n"
		}
		d.Notes += f.appendText + "\n"
	default:
		edited, err := openEditor(d.Notes)
		if err != nil {
			return err
		}
		d.Notes = edited
	}

	d.ModifiedAt = time.Now().UTC()
	if err := drafts.Store().Save(d); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Notes updated for draft %s/%s.\n",
		weekStart.Format("2006-01-02"), name)
	return nil
}

func openEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	file, err := os.CreateTemp("", "tdx-note-*.txt")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(initial); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	file.Close()
	defer os.Remove(file.Name())

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
