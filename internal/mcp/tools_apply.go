package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// dayNames maps short day names to time.Weekday for parseDaysFilter.
var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

type compareArgs struct {
	Name    string `json:"name" jsonschema:"required,description=template name"`
	Week    string `json:"week" jsonschema:"required,description=any date in target week YYYY-MM-DD"`
	Mode    string `json:"mode,omitempty" jsonschema:"description=apply mode: add (default) / replace-matching / replace-mine"`
	Days    string `json:"days,omitempty" jsonschema:"description=day filter: mon-thu or mon,wed,fri"`
	Profile string `json:"profile,omitempty"`
}

type previewArgs struct {
	Name      string   `json:"name" jsonschema:"required,description=template name"`
	Week      string   `json:"week" jsonschema:"required,description=target week YYYY-MM-DD"`
	Mode      string   `json:"mode,omitempty"`
	Days      string   `json:"days,omitempty"`
	Overrides []string `json:"overrides,omitempty" jsonschema:"description=hour overrides e.g. row-01:fri=4"`
	Profile   string   `json:"profile,omitempty"`
}

// RegisterApplyTools registers compare, preview, and apply MCP tools.
func RegisterApplyTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "compare_template_to_week",
		Description: "Compare a time template against a week to see what changes would be made.",
	}, compareHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "preview_apply_time_template",
		Description: "Preview applying a template to a week with optional overrides. Returns a diffHash for apply.",
	}, previewHandler(svcs))
}

// reconcileResult is the JSON shape returned by compare and preview tools.
type reconcileResult struct {
	Actions  []domain.Action  `json:"actions"`
	Blockers []domain.Blocker `json:"blockers"`
	Creates  int              `json:"creates"`
	Updates  int              `json:"updates"`
	Skips    int              `json:"skips"`
	DiffHash string           `json:"diffHash"`
}

func compareHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, compareArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args compareArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		tmpl, err := svcs.Template.Store().Load(args.Name)
		if err != nil {
			return errorResult(fmt.Sprintf("load template: %v", err)), nil, nil
		}

		weekDate, err := time.ParseInLocation("2006-01-02", args.Week, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid week date: %v", err)), nil, nil
		}
		weekRef := domain.WeekRefContaining(weekDate)

		mode, err := parseMode(args.Mode)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		daysFilter, err := parseDaysFilter(args.Days)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		input := tmplsvc.ReconcileInput{
			Template:   tmpl,
			WeekRef:    weekRef,
			Mode:       mode,
			DaysFilter: daysFilter,
			Round:      true,
			UserUID:    user.UID,
		}

		diff, err := svcs.Template.Reconcile(ctx, profile, input)
		if err != nil {
			return errorResult(fmt.Sprintf("reconcile: %v", err)), nil, nil
		}

		creates, updates, skips := diff.CountByKind()
		return jsonResult(reconcileResult{
			Actions:  diff.Actions,
			Blockers: diff.Blockers,
			Creates:  creates,
			Updates:  updates,
			Skips:    skips,
			DiffHash: diff.DiffHash,
		})
	}
}

func previewHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, previewArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args previewArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		tmpl, err := svcs.Template.Store().Load(args.Name)
		if err != nil {
			return errorResult(fmt.Sprintf("load template: %v", err)), nil, nil
		}

		weekDate, err := time.ParseInLocation("2006-01-02", args.Week, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid week date: %v", err)), nil, nil
		}
		weekRef := domain.WeekRefContaining(weekDate)

		mode, err := parseMode(args.Mode)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		daysFilter, err := parseDaysFilter(args.Days)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		overrides, err := parseOverrides(args.Overrides)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		input := tmplsvc.ReconcileInput{
			Template:   tmpl,
			WeekRef:    weekRef,
			Mode:       mode,
			DaysFilter: daysFilter,
			Overrides:  overrides,
			Round:      true,
			UserUID:    user.UID,
		}

		diff, err := svcs.Template.Reconcile(ctx, profile, input)
		if err != nil {
			return errorResult(fmt.Sprintf("reconcile: %v", err)), nil, nil
		}

		creates, updates, skips := diff.CountByKind()
		return jsonResult(reconcileResult{
			Actions:  diff.Actions,
			Blockers: diff.Blockers,
			Creates:  creates,
			Updates:  updates,
			Skips:    skips,
			DiffHash: diff.DiffHash,
		})
	}
}

// parseMode parses the apply mode string, defaulting to "add".
func parseMode(s string) (domain.ApplyMode, error) {
	if s == "" {
		return domain.ModeAdd, nil
	}
	return domain.ParseApplyMode(s)
}

// parseDaysFilter parses a day filter string like "mon-thu" or "mon,wed,fri".
func parseDaysFilter(s string) ([]time.Weekday, error) {
	if s == "" {
		return nil, nil
	}
	// Handle range: "mon-thu"
	if strings.Contains(s, "-") && !strings.Contains(s, ",") {
		parts := strings.SplitN(s, "-", 2)
		start, ok1 := dayNames[strings.TrimSpace(strings.ToLower(parts[0]))]
		end, ok2 := dayNames[strings.TrimSpace(strings.ToLower(parts[1]))]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid day range: %s", s)
		}
		var days []time.Weekday
		for d := start; d <= end; d++ {
			days = append(days, d)
		}
		return days, nil
	}
	// Handle list: "mon,wed,fri"
	var days []time.Weekday
	for _, name := range strings.Split(s, ",") {
		d, ok := dayNames[strings.TrimSpace(strings.ToLower(name))]
		if !ok {
			return nil, fmt.Errorf("unknown day: %s", strings.TrimSpace(name))
		}
		days = append(days, d)
	}
	return days, nil
}

// parseOverrides parses override strings like "row-01:fri=4" into tmplsvc.Override.
func parseOverrides(strs []string) ([]tmplsvc.Override, error) {
	var out []tmplsvc.Override
	for _, s := range strs {
		colonIdx := strings.Index(s, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid override format: %s (expected row:day=hours)", s)
		}
		rowID := s[:colonIdx]
		rest := s[colonIdx+1:]
		eqIdx := strings.Index(rest, "=")
		if eqIdx < 0 {
			return nil, fmt.Errorf("invalid override format: %s", s)
		}
		dayName := rest[:eqIdx]
		hoursStr := rest[eqIdx+1:]
		day, ok := dayNames[strings.ToLower(dayName)]
		if !ok {
			return nil, fmt.Errorf("unknown day in override: %s", dayName)
		}
		hours, err := strconv.ParseFloat(hoursStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid hours in override: %s", hoursStr)
		}
		out = append(out, tmplsvc.Override{RowID: rowID, Day: day, Hours: hours})
	}
	return out, nil
}
