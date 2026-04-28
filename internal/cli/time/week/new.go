package week

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

type newFlags struct {
	profile      string
	name         string
	fromTemplate string
	fromDraft    string
	shift        string
	json         bool
}

type weekDraftCreateResult struct {
	Schema string           `json:"schema"`
	Draft  domain.WeekDraft `json:"draft"`
}

func newNewCmd() *cobra.Command {
	var f newFlags
	cmd := &cobra.Command{
		Use:   "new <date>",
		Short: "Create a blank, template-seeded, or draft-cloned week draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.name, "name", "", "draft name (default: default)")
	cmd.Flags().StringVar(&f.fromTemplate, "from-template", "", "seed rows from this template")
	cmd.Flags().StringVar(&f.fromDraft, "from-draft", "", "clone this draft (date or date/name)")
	cmd.Flags().StringVar(&f.shift, "shift", "", "with --from-draft, shift the source by this duration (e.g. 7d, -7d)")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runNew(cmd *cobra.Command, f newFlags, dateRef string) error {
	weekStart, name, err := ParseDraftRef(dateRef)
	if err != nil {
		return err
	}
	if f.name != "" {
		name = f.name
	}

	if f.fromTemplate != "" && f.fromDraft != "" {
		return fmt.Errorf("--from-template and --from-draft are mutually exclusive")
	}
	if f.shift != "" && f.fromDraft == "" {
		return fmt.Errorf("--shift requires --from-draft")
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

	var draft domain.WeekDraft
	switch {
	case f.fromTemplate != "":
		tmplStore := tmplsvc.NewStore(paths)
		tmpl, err := tmplStore.Load(profileName, f.fromTemplate)
		if err != nil {
			return err
		}
		draft, err = drafts.NewFromTemplate(profileName, weekStart, name, tmpl)
		if err != nil {
			return err
		}
	case f.fromDraft != "":
		srcWeek, srcName, err := ParseDraftRef(f.fromDraft)
		if err != nil {
			return err
		}
		if f.shift != "" {
			d, err := parseShift(f.shift)
			if err != nil {
				return err
			}
			// --shift Nd means: source is at (weekStart - Nd), produce a draft at weekStart.
			srcWeek = weekStart.Add(-d)
		}
		draft, err = drafts.NewFromDraft(profileName, weekStart, name, profileName, srcWeek, srcName)
		if err != nil {
			return err
		}
	default:
		draft, err = drafts.NewBlank(profileName, weekStart, name)
		if err != nil {
			return err
		}
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeNewResultJSON(w, draft)
	}
	_, _ = fmt.Fprintf(w, "Created draft %s/%s.\n",
		weekStart.Format("2006-01-02"), draft.Name)
	return nil
}

func writeNewResultJSON(w io.Writer, d domain.WeekDraft) error {
	return json.NewEncoder(w).Encode(weekDraftCreateResult{
		Schema: "tdx.v1.weekDraftCreateResult", Draft: d,
	})
}

// parseShift accepts strings like "7d", "-7d", "14d".
func parseShift(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "d") {
		return 0, fmt.Errorf("--shift must end in 'd' (e.g. 7d, -7d), got %q", s)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
	if err != nil {
		return 0, fmt.Errorf("invalid --shift %q: %w", s, err)
	}
	return time.Duration(n) * 24 * time.Hour, nil
}
