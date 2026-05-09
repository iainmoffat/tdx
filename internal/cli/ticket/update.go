package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// rawUpdateFlags captures the raw cobra flag values plus a boolean for
// each field indicating whether the flag was explicitly set
// (cmd.Flags().Changed(name)). The "set" booleans are needed because Title
// and Description accept empty strings as valid values; we can't use empty
// to mean "not provided".
type rawUpdateFlags struct {
	title          string
	titleSet       bool
	description    string
	descriptionSet bool
	typeArg        string // numeric or name; empty = not set
	accountArg     string
	requestorArg   string
	groupArg       string
	priorityID     int // 0 = not set
	prioritySet    bool
	comment        string
}

// ticketUpdateFields holds the resolved field values to PATCH. Pointers
// distinguish "set to value X" (including empty strings) from "don't touch".
type ticketUpdateFields struct {
	title        *string
	description  *string
	typeID       *int
	accountID    *int
	requestorUID *string
	groupID      *int
	priorityID   *int
}

// hasAny reports whether at least one field is set.
func (f ticketUpdateFields) hasAny() bool {
	return f.title != nil || f.description != nil || f.typeID != nil ||
		f.accountID != nil || f.requestorUID != nil || f.groupID != nil ||
		f.priorityID != nil
}

// buildTicketPatchOps emits a JSON-Patch op per non-nil field. Order is
// stable for testability: Title, Description, TypeID, AccountID,
// RequestorUid, ResponsibleGroupID, PriorityID.
func buildTicketPatchOps(f ticketUpdateFields) []ticketsvc.PatchOp {
	var ops []ticketsvc.PatchOp
	if f.title != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Title", Value: *f.title})
	}
	if f.description != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Description", Value: *f.description})
	}
	if f.typeID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/TypeID", Value: *f.typeID})
	}
	if f.accountID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/AccountID", Value: *f.accountID})
	}
	if f.requestorUID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/RequestorUid", Value: *f.requestorUID})
	}
	if f.groupID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/ResponsibleGroupID", Value: *f.groupID})
	}
	if f.priorityID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/PriorityID", Value: *f.priorityID})
	}
	return ops
}

// resolveUpdateFields resolves rawUpdateFlags to ticketUpdateFields,
// using the appropriate name → ID resolver per field.
func resolveUpdateFields(ctx context.Context, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID int, raw rawUpdateFlags) (ticketUpdateFields, error) {
	var out ticketUpdateFields

	if raw.titleSet {
		v := raw.title
		out.title = &v
	}
	if raw.descriptionSet {
		v := raw.description
		out.description = &v
	}
	if raw.typeArg != "" {
		id, name := parseStatusArg(raw.typeArg)
		if id > 0 {
			v := id
			out.typeID = &v
		} else {
			tt, err := svc.ResolveTypeByName(ctx, profile, appID, name)
			if err != nil {
				return out, fmt.Errorf("--type %q: %w", raw.typeArg, err)
			}
			v := tt.ID
			out.typeID = &v
		}
	}
	if raw.accountArg != "" {
		id, name := parseStatusArg(raw.accountArg)
		if id > 0 {
			v := id
			out.accountID = &v
		} else {
			acct, err := people.ResolveAccountByName(ctx, profile, name)
			if err != nil {
				return out, fmt.Errorf("--account %q: %w", raw.accountArg, err)
			}
			v := acct.ID
			out.accountID = &v
		}
	}
	if raw.requestorArg != "" {
		uid, err := resolvePrincipal(ctx, people, profile, authedUID, raw.requestorArg)
		if err != nil {
			return out, fmt.Errorf("--requestor %q: %w", raw.requestorArg, err)
		}
		out.requestorUID = &uid
	}
	if raw.groupArg != "" {
		id, name := parseStatusArg(raw.groupArg)
		if id > 0 {
			v := id
			out.groupID = &v
		} else {
			g, err := svc.ResolveGroupByName(ctx, profile, name)
			if err != nil {
				return out, fmt.Errorf("--responsibility-group %q: %w", raw.groupArg, err)
			}
			v := g.ID
			out.groupID = &v
		}
	}
	if raw.prioritySet {
		v := raw.priorityID
		out.priorityID = &v
	}

	return out, nil
}

// changedFieldsSummary builds the human-readable summary of which fields
// changed. For long values like description, prints "<changed>" rather
// than echoing the full text. ID-only fields fall back to "<key>-id=N"
// when the post-patch ticket lacks the resolved name.
func changedFieldsSummary(f ticketUpdateFields, ticketAfterPatch domain.Ticket) string {
	parts := []string{}
	if f.title != nil {
		parts = append(parts, fmt.Sprintf("title=%q", truncate(*f.title, 60)))
	}
	if f.description != nil {
		parts = append(parts, "description=<changed>")
	}
	if f.typeID != nil {
		name := ticketAfterPatch.TypeName
		if name == "" {
			parts = append(parts, fmt.Sprintf("type-id=%d", *f.typeID))
		} else {
			parts = append(parts, fmt.Sprintf("type=%s", name))
		}
	}
	if f.accountID != nil {
		name := ticketAfterPatch.AccountName
		if name == "" {
			parts = append(parts, fmt.Sprintf("account-id=%d", *f.accountID))
		} else {
			parts = append(parts, fmt.Sprintf("account=%s", name))
		}
	}
	if f.requestorUID != nil {
		name := ticketAfterPatch.RequestorName
		if name == "" {
			parts = append(parts, "requestor=<changed>")
		} else {
			parts = append(parts, fmt.Sprintf("requestor=%s", name))
		}
	}
	if f.groupID != nil {
		parts = append(parts, fmt.Sprintf("responsibility-group-id=%d", *f.groupID))
	}
	if f.priorityID != nil {
		name := ticketAfterPatch.PriorityName
		if name == "" {
			parts = append(parts, fmt.Sprintf("priority-id=%d", *f.priorityID))
		} else {
			parts = append(parts, fmt.Sprintf("priority=%s", name))
		}
	}
	return strings.Join(parts, ", ")
}

func newUpdateCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		titleFlag   string
		descFlag    string
		typeArg     string
		accountArg  string
		requestArg  string
		groupArg    string
		priorityID  int
		commentFlag string
		yesFlag     bool
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update editable ticket fields (title/description/type/account/requestor/group/priority); --yes required",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to update the ticket")
			}
			raw := rawUpdateFlags{
				title:          titleFlag,
				titleSet:       cmd.Flags().Changed("title"),
				description:    descFlag,
				descriptionSet: cmd.Flags().Changed("description"),
				typeArg:        typeArg,
				accountArg:     accountArg,
				requestorArg:   requestArg,
				groupArg:       groupArg,
				priorityID:     priorityID,
				prioritySet:    cmd.Flags().Changed("priority-id"),
				comment:        commentFlag,
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
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}
			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			people := peoplesvc.New(paths)
			return runTicketUpdate(cmd.Context(), cmd.OutOrStdout(), s, people, profile, authedUID, appID, id, raw, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().StringVar(&titleFlag, "title", "", "set ticket title")
	cmd.Flags().StringVar(&descFlag, "description", "", "set ticket description (replaces existing)")
	cmd.Flags().StringVar(&typeArg, "type", "", "set ticket type by name or id")
	cmd.Flags().StringVar(&accountArg, "account", "", "set account by name or id")
	cmd.Flags().StringVar(&requestArg, "requestor", "", "set requestor by uid|email|me")
	cmd.Flags().StringVar(&groupArg, "responsibility-group", "", "set responsibility group by name or id")
	cmd.Flags().IntVar(&priorityID, "priority-id", 0, "set priority by id (numeric only this round)")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "optional accompanying feed comment (posted after PATCH succeeds)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to mutate")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketUpdate(ctx context.Context, w io.Writer, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID, id int, raw rawUpdateFlags, jsonOut bool) error {
	fields, err := resolveUpdateFields(ctx, svc, people, profile, authedUID, appID, raw)
	if err != nil {
		return err
	}
	if !fields.hasAny() && raw.comment == "" {
		return fmt.Errorf("nothing to update — pass at least one of --title / --description / --type / --account / --requestor / --responsibility-group / --priority-id / --comment")
	}
	var updated domain.Ticket
	if fields.hasAny() {
		ops := buildTicketPatchOps(fields)
		updated, err = svc.PatchTicket(ctx, profile, appID, id, ops)
		if err != nil {
			return err
		}
	} else {
		updated, err = svc.GetTicket(ctx, profile, appID, id)
		if err != nil {
			return err
		}
	}

	commentNote := ""
	if raw.comment != "" {
		feedID, ferr := svc.AddFeed(ctx, profile, appID, id, raw.comment, false, nil)
		if ferr != nil {
			commentNote = fmt.Sprintf(" (warning: comment failed: %v)", ferr)
		} else {
			commentNote = fmt.Sprintf(" (comment posted: feed entry %d)", feedID)
		}
	}

	if jsonOut {
		return render.JSON(w, struct {
			Schema string        `json:"schema"`
			Ticket domain.Ticket `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: updated})
	}

	summary := changedFieldsSummary(fields, updated)
	if summary == "" {
		_, _ = fmt.Fprintf(w, "ticket #%d: no field changes%s\n", id, commentNote)
	} else {
		_, _ = fmt.Fprintf(w, "ticket #%d updated: %s%s\n", id, summary, commentNote)
	}
	return nil
}
