package mcp

import (
	"context"
	"fmt"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

type searchPeopleArgs struct {
	Profile        string `json:"profile,omitempty"`
	SearchText     string `json:"searchText"`
	MaxResults     int    `json:"maxResults,omitempty"`
	IncludeClients bool   `json:"includeClients,omitempty" jsonschema:"include portal clients (default false: staff only)"`
}

type getPersonArgs struct {
	Profile string `json:"profile,omitempty"`
	UID     string `json:"uid"`
}

type listAccountsArgs struct {
	Profile string `json:"profile,omitempty"`
}

type listResourcePoolsArgs struct {
	Profile string `json:"profile,omitempty"`
}

// RegisterPeopleTools registers the read-only people-discovery tools.
func RegisterPeopleTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "search_people",
		Description: `Find people by partial name, email, or ID via TD's autocomplete endpoint.

Defaults to staff only (IsEmployee=true). Set includeClients=true to include
portal users.

Read-only — no confirm required.`,
	}, searchPeopleHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_person",
		Description: "Fetch full details for a single user by UID. Read-only — no confirm required.",
	}, getPersonHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_accounts",
		Description: "List TD accounts/departments. Sorted by name. Read-only — no confirm required.",
	}, listAccountsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_resource_pools",
		Description: "List TD resource pools. Sorted by name. Read-only — no confirm required.",
	}, listResourcePoolsHandler(svcs))
}

type personJSON struct {
	UID        string `json:"uid"`
	FullName   string `json:"fullName"`
	Email      string `json:"email,omitempty"`
	Account    string `json:"account,omitempty"`
	Manager    string `json:"manager,omitempty"`
	IsEmployee bool   `json:"isEmployee"`
	IsActive   bool   `json:"isActive"`
	Title      string `json:"title,omitempty"`
}

func toPersonJSON(u domain.User) personJSON {
	return personJSON{
		UID:        u.UID,
		FullName:   u.FullName,
		Email:      u.Email,
		Account:    u.AccountName,
		Manager:    u.ReportsToName,
		IsEmployee: u.IsEmployee,
		IsActive:   u.Active,
		Title:      u.Title,
	}
}

func searchPeopleHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, searchPeopleArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args searchPeopleArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		users, err := svcs.People.LookupPeople(ctx, profile, args.SearchText, args.MaxResults)
		if err != nil {
			return errorResult(fmt.Sprintf("search_people: %v", err)), nil, nil
		}
		out := make([]personJSON, 0, len(users))
		for _, u := range users {
			if !args.IncludeClients && !u.IsEmployee {
				continue
			}
			out = append(out, toPersonJSON(u))
		}
		return jsonResult(struct {
			Schema string       `json:"schema"`
			Query  string       `json:"query"`
			People []personJSON `json:"people"`
		}{
			Schema: "tdx.v1.peopleSearchResult",
			Query:  args.SearchText,
			People: out,
		})
	}
}

func getPersonHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getPersonArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getPersonArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		user, err := svcs.People.GetUser(ctx, profile, args.UID)
		if err != nil {
			return errorResult(fmt.Sprintf("get_person: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string      `json:"schema"`
			Person domain.User `json:"person"`
		}{
			Schema: "tdx.v1.person",
			Person: user,
		})
	}
}

type accountJSON struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	IsActive        bool   `json:"isActive"`
	Code            string `json:"code,omitempty"`
	ManagerUID      string `json:"managerUID,omitempty"`
	ManagerFullName string `json:"managerFullName,omitempty"`
}

func listAccountsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listAccountsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listAccountsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		accounts, err := svcs.People.SearchAccounts(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("list_accounts: %v", err)), nil, nil
		}
		sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
		out := make([]accountJSON, 0, len(accounts))
		for _, a := range accounts {
			out = append(out, accountJSON{
				ID:              a.ID,
				Name:            a.Name,
				IsActive:        a.IsActive,
				Code:            a.Code,
				ManagerUID:      a.ManagerUID,
				ManagerFullName: a.ManagerFullName,
			})
		}
		return jsonResult(struct {
			Schema   string        `json:"schema"`
			Accounts []accountJSON `json:"accounts"`
		}{
			Schema:   "tdx.v1.accountList",
			Accounts: out,
		})
	}
}

type resourcePoolJSON struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	IsActive         bool   `json:"isActive"`
	RequiresApproval bool   `json:"requiresApproval"`
	ManagerUID       string `json:"managerUID,omitempty"`
	ManagerFullName  string `json:"managerFullName,omitempty"`
}

func listResourcePoolsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listResourcePoolsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listResourcePoolsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		pools, err := svcs.People.SearchPools(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("list_resource_pools: %v", err)), nil, nil
		}
		sort.SliceStable(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })
		out := make([]resourcePoolJSON, 0, len(pools))
		for _, p := range pools {
			out = append(out, resourcePoolJSON{
				ID:               p.ID,
				Name:             p.Name,
				IsActive:         p.IsActive,
				RequiresApproval: p.RequiresApproval,
				ManagerUID:       p.ManagerUID,
				ManagerFullName:  p.ManagerFullName,
			})
		}
		return jsonResult(struct {
			Schema string             `json:"schema"`
			Pools  []resourcePoolJSON `json:"pools"`
		}{
			Schema: "tdx.v1.resourcePoolList",
			Pools:  out,
		})
	}
}

// _ peoplesvc reference to keep the import used even if a handler is later
// removed. Currently used by SearchAccounts/SearchPools return types via
// svcs.People; this is just a compile-time anchor.
var _ = peoplesvc.Account{}
