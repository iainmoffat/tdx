package domain

import (
	"errors"
	"fmt"
)

// ErrInvalidTarget is returned by Target.Validate when the target is
// structurally incomplete or has an unknown Kind.
var ErrInvalidTarget = errors.New("invalid target")

// TargetKind enumerates the work-item kinds tdx understands. Each value
// corresponds to one or more TD TimeEntryComponent values during decoding.
type TargetKind string

const (
	TargetTicket       TargetKind = "ticket"
	TargetTicketTask   TargetKind = "ticketTask"
	TargetProject      TargetKind = "project"
	TargetProjectTask  TargetKind = "projectTask"
	TargetProjectIssue TargetKind = "projectIssue"
	TargetWorkspace    TargetKind = "workspace"
	TargetTimeOff      TargetKind = "timeoff"
	TargetPortfolio    TargetKind = "portfolio"
	TargetRequest      TargetKind = "request"
)

// IsKnown reports whether k is one of the declared TargetKind constants.
func (k TargetKind) IsKnown() bool {
	switch k {
	case TargetTicket, TargetTicketTask, TargetProject, TargetProjectTask,
		TargetProjectIssue, TargetWorkspace, TargetTimeOff, TargetPortfolio,
		TargetRequest:
		return true
	}
	return false
}

// SupportsComponentLookup reports whether TD exposes a
// /TDWebApi/api/time/types/component/... endpoint for this kind that tdx
// can currently address. Only kinds that return true can be passed to
// `tdx time type for`. (TargetProjectTask is intentionally excluded
// until domain.Target gains a PlanID field.)
func (k TargetKind) SupportsComponentLookup() bool {
	switch k {
	case TargetTicket, TargetTicketTask,
		TargetProject, TargetProjectIssue,
		TargetWorkspace, TargetTimeOff, TargetRequest:
		return true
	}
	return false
}

// Target captures everything needed to address a TD work item for time
// operations. DisplayName and DisplayRef are for rendering only.
type Target struct {
	Kind        TargetKind `json:"kind" yaml:"kind"`
	AppID       int        `json:"appID" yaml:"appID"`
	ItemID      int        `json:"itemID" yaml:"itemID"`
	TaskID      int        `json:"taskID,omitempty" yaml:"taskID,omitempty"`
	DisplayName string     `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DisplayRef  string     `json:"displayRef,omitempty" yaml:"displayRef,omitempty"`
}

// Validate returns nil if the target is structurally sound for an API call.
func (t Target) Validate() error {
	if t.Kind == "" {
		return fmt.Errorf("%w: kind is required", ErrInvalidTarget)
	}
	if !t.Kind.IsKnown() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidTarget, t.Kind)
	}
	if t.AppID <= 0 {
		return fmt.Errorf("%w: appID is required", ErrInvalidTarget)
	}
	if t.ItemID <= 0 {
		return fmt.Errorf("%w: itemID is required", ErrInvalidTarget)
	}
	if (t.Kind == TargetTicketTask || t.Kind == TargetProjectTask) && t.TaskID <= 0 {
		return fmt.Errorf("%w: %s requires taskID", ErrInvalidTarget, t.Kind)
	}
	return nil
}
