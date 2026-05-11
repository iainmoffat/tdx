package ticketsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetFeed fetches feed entries for a ticket. Order is whatever TD returns
// (typically newest-first); CLI layer may re-sort if needed.
func (s *Service) GetFeed(ctx context.Context, profileName string, appID, ticketID int) ([]domain.TicketFeedEntry, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/feed", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get ticket feed %d: %w", ticketID, err)
	}
	out := make([]domain.TicketFeedEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketFeedEntry{
			ID:         w.ID,
			AuthorUID:  w.CreatedUid,
			AuthorName: w.CreatedFullName,
			CreatedAt:  parseTDTime(w.CreatedDate),
			Body:       w.Body,
			IsPrivate:  w.IsPrivate,
			EventKind:  classifyFeedKind(w.UpdateType),
		})
	}
	return out, nil
}

// AddFeed posts a comment to a ticket. Returns the created entry's ID.
// notify is a list of UIDs to notify in addition to default recipients.
func (s *Service) AddFeed(ctx context.Context, profileName string, appID, ticketID int, body string, isPrivate bool, notify []string) (int, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return 0, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireFeedAdd{
		Comments:  body,
		Notify:    notify,
		IsPrivate: isPrivate,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/feed", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("add ticket feed %d: %w", ticketID, err)
	}
	return resp.ID, nil
}

// classifyFeedKind maps TD's UpdateType integer to a human label.
//
// Live-verified 2026-05-08: comments come through as UpdateType=1;
// system events (status changes, assignment changes, attachments, etc.)
// all come through as UpdateType=3. The body text already describes what
// the event is, so we label class 3 generically as "event" rather than
// pretending we can distinguish status-change from attachment from
// assignment without parsing the body.
func classifyFeedKind(t int) string {
	switch t {
	case 1:
		return "comment"
	default:
		return "event"
	}
}
