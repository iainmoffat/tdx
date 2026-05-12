package projectsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetFeed fetches feed entries for a project.
func (s *Service) GetFeed(ctx context.Context, profileName string, projectID int) ([]domain.ProjectFeedEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/feed", projectID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get project feed %d: %w", projectID, err)
	}
	out := make([]domain.ProjectFeedEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeFeedEntry(w))
	}
	return out, nil
}

// AddFeed posts a comment to a project. Returns the created entry's ID.
func (s *Service) AddFeed(ctx context.Context, profileName string, projectID int, message string, isPrivate bool, notify []string) (int, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireProjectFeedAdd{
		Body:      message,
		Notify:    notify,
		IsPrivate: isPrivate,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/feed", projectID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("add project feed %d: %w", projectID, err)
	}
	return resp.ID, nil
}

// GetTaskFeed fetches feed entries for a project task.
func (s *Service) GetTaskFeed(ctx context.Context, profileName string, projectID, planID, taskID int) ([]domain.ProjectFeedEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/plans/%d/tasks/%d/feed", projectID, planID, taskID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get project task feed %d/%d/%d: %w", projectID, planID, taskID, err)
	}
	out := make([]domain.ProjectFeedEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeFeedEntry(w))
	}
	return out, nil
}

// AddTaskFeed posts a comment to a project task. Returns the created entry's ID.
func (s *Service) AddTaskFeed(ctx context.Context, profileName string, projectID, planID, taskID int, message string, isPrivate bool, notify []string) (int, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireTaskFeedAdd{
		Comments:  message,
		Notify:    notify,
		IsPrivate: isPrivate,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/plans/%d/tasks/%d/feed", projectID, planID, taskID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("add project task feed %d/%d/%d: %w", projectID, planID, taskID, err)
	}
	return resp.ID, nil
}

// decodeFeedEntry maps a wire feed entry to the domain type.
func decodeFeedEntry(w wireFeedEntry) domain.ProjectFeedEntry {
	return domain.ProjectFeedEntry{
		ID:              w.ID,
		Body:            w.Body,
		CreatedByUID:    w.CreatedUid,
		CreatedByName:   w.CreatedFullName,
		CreatedDate:     parseTD(w.CreatedDate),
		LastUpdatedDate: parseTD(w.LastUpdatedDate),
		UpdateType:      w.UpdateType,
		IsPrivate:       w.IsPrivate,
		LikesCount:      w.LikesCount,
		RepliesCount:    w.RepliesCount,
	}
}
