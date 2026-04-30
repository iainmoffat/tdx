package peoplesvc

import (
	"context"
	"fmt"
	"strings"
)

// ResourcePool is a TD resource pool as returned by /api/resourcepools/search.
type ResourcePool struct {
	ID               int
	Name             string
	IsActive         bool
	RequiresApproval bool
	ManagerUID       string
	ManagerFullName  string
}

// SearchPools lists all resource pools visible to the profile.
// TD's /api/resourcepools/search ignores filter inputs in practice, so we
// always send {} and decode the full list.
func (s *Service) SearchPools(ctx context.Context, profileName string) ([]ResourcePool, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireResourcePool
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/resourcepools/search", struct{}{}, &wire); err != nil {
		return nil, fmt.Errorf("search resource pools: %w", err)
	}
	out := make([]ResourcePool, 0, len(wire))
	for _, w := range wire {
		out = append(out, ResourcePool{
			ID:               w.ID,
			Name:             strings.TrimSpace(w.Name),
			IsActive:         w.IsActive,
			RequiresApproval: w.RequiresApproval,
			ManagerUID:       w.ManagerUID,
			ManagerFullName:  w.ManagerFullName,
		})
	}
	return out, nil
}

// ResolvePoolByName looks up a single resource pool by case-insensitive name
// (after trimming whitespace). Returns an error when no pool matches or when
// multiple pools share the name.
func (s *Service) ResolvePoolByName(ctx context.Context, profileName, name string) (ResourcePool, error) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return ResourcePool{}, fmt.Errorf("resource pool name is empty")
	}
	pools, err := s.SearchPools(ctx, profileName)
	if err != nil {
		return ResourcePool{}, err
	}
	matches := []ResourcePool{}
	for _, p := range pools {
		if strings.ToLower(p.Name) == target {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return ResourcePool{}, fmt.Errorf("resource pool %q not found among %d pools", name, len(pools))
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, fmt.Sprintf("%d", m.ID))
		}
		return ResourcePool{}, fmt.Errorf("resource pool %q is ambiguous (matched IDs %s)", name, strings.Join(ids, ", "))
	}
}
