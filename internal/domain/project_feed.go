package domain

import "time"

// ProjectFeedEntry is one row from GET /api/projects/{id}/feed or
// /api/projects/{p}/plans/{pl}/tasks/{t}/feed. UpdateType: 1=comment, 3=system.
type ProjectFeedEntry struct {
	ID              int       `json:"id"`
	Body            string    `json:"body"`
	CreatedByUID    string    `json:"createdByUID,omitempty"`
	CreatedByName   string    `json:"createdByName,omitempty"`
	CreatedDate     time.Time `json:"createdDate,omitempty"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate,omitempty"`
	UpdateType      int       `json:"updateTypeID,omitempty"`
	IsPrivate       bool      `json:"isPrivate,omitempty"`
	LikesCount      int       `json:"likesCount,omitempty"`
	RepliesCount    int       `json:"repliesCount,omitempty"`
}

// UpdateTypeLabel returns "comment" for UpdateType==1, "system" otherwise.
func (e ProjectFeedEntry) UpdateTypeLabel() string {
	if e.UpdateType == 1 {
		return "comment"
	}
	return "system"
}
