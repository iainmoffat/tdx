package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProjectFeedEntry_UpdateTypeLabel(t *testing.T) {
	tests := []struct {
		updateType int
		want       string
	}{
		{1, "comment"},
		{3, "system"},
		{0, "system"},
		{99, "system"},
	}
	for _, tc := range tests {
		e := ProjectFeedEntry{UpdateType: tc.updateType}
		if got := e.UpdateTypeLabel(); got != tc.want {
			t.Errorf("UpdateType=%d: got %q, want %q", tc.updateType, got, tc.want)
		}
	}
}

func TestProjectFeedEntry_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 30, 0, 0, time.UTC)
	e := ProjectFeedEntry{
		ID:              1782210,
		Body:            "Changed Portfolio(s) from \"\" to \"Sample\".",
		CreatedByUID:    "aaaa-bbbb-cccc-dddd",
		CreatedByName:   "Pat Manager",
		CreatedDate:     now,
		LastUpdatedDate: now,
		UpdateType:      3,
		IsPrivate:       false,
		LikesCount:      0,
		RepliesCount:    2,
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got ProjectFeedEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID {
		t.Errorf("ID: got %d, want %d", got.ID, e.ID)
	}
	if got.Body != e.Body {
		t.Errorf("Body: got %q, want %q", got.Body, e.Body)
	}
	if got.CreatedByUID != e.CreatedByUID {
		t.Errorf("CreatedByUID: got %q, want %q", got.CreatedByUID, e.CreatedByUID)
	}
	if got.UpdateType != e.UpdateType {
		t.Errorf("UpdateType: got %d, want %d", got.UpdateType, e.UpdateType)
	}
	if got.RepliesCount != e.RepliesCount {
		t.Errorf("RepliesCount: got %d, want %d", got.RepliesCount, e.RepliesCount)
	}
}
