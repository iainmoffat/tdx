package peoplesvc

import (
	"context"
	"fmt"
	"net/url"

	"github.com/iainmoffat/tdx/internal/domain"
)

// lookupDefaultMax is the default cap on /api/people/lookup results when
// the caller passes 0 (or anything <= 0).
const lookupDefaultMax = 25

// lookupHardCap is the upper bound enforced on every lookup call. TD itself
// caps responses, but we add our own ceiling to keep payload size bounded
// for CLI-driven invocations.
const lookupHardCap = 100

// LookupPeople searches by partial name/email/ID via the autocomplete
// endpoint GET /api/people/lookup. The /search endpoint silently ignores
// most filter params (NameLike, ReportsToUid, etc.); /lookup is the right
// tool when you want "find this person."
//
// maxResults <= 0 defaults to 25; values above 100 are clamped.
func (s *Service) LookupPeople(ctx context.Context, profileName, searchText string, maxResults int) ([]domain.User, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = lookupDefaultMax
	}
	if maxResults > lookupHardCap {
		maxResults = lookupHardCap
	}
	q := url.Values{}
	q.Set("searchText", searchText)
	q.Set("maxResults", fmt.Sprintf("%d", maxResults))
	path := "/TDWebApi/api/people/lookup?" + q.Encode()

	var wire []wireUser
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("lookup people: %w", err)
	}
	out := make([]domain.User, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeUser(w))
	}
	return out, nil
}
