package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
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
	comment        string //nolint:unused // wired in Task 3 cobra cmd
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
