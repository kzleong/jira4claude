package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kzleong/jira4claude"
)

// BoardService implements jira4claude.BoardService using the Jira Agile REST API.
type BoardService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.BoardService = (*BoardService)(nil)

// NewBoardService creates a new BoardService using the provided HTTP client.
func NewBoardService(client *Client) *BoardService {
	return &BoardService{client: client}
}

// ListBoards returns boards associated with the given project key.
func (s *BoardService) ListBoards(ctx context.Context, project string) ([]*jira4claude.Board, error) {
	path := "/rest/agile/1.0/board"
	if project != "" {
		path += "?projectKeyOrId=" + url.QueryEscape(project)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp boardListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	boards := make([]*jira4claude.Board, len(resp.Values))
	for i, b := range resp.Values {
		boards[i] = &jira4claude.Board{
			ID:   b.ID,
			Name: b.Name,
			Type: b.Type,
		}
	}
	return boards, nil
}

// SprintService implements jira4claude.SprintService using the Jira Agile REST API.
type SprintService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.SprintService = (*SprintService)(nil)

// NewSprintService creates a new SprintService using the provided HTTP client.
func NewSprintService(client *Client) *SprintService {
	return &SprintService{client: client}
}

// ListSprints returns sprints on the given board.
// If states is empty, active and future sprints are returned.
func (s *SprintService) ListSprints(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error) {
	stateParam := "active,future"
	if len(states) > 0 {
		stateParam = strings.Join(states, ",")
	}
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=%s", boardID, url.QueryEscape(stateParam))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp sprintListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	sprints := make([]*jira4claude.Sprint, len(resp.Values))
	for i, sp := range resp.Values {
		sprints[i] = sp.toSprint()
	}
	return sprints, nil
}

// boardListResponse represents the Jira Agile API board list response.
type boardListResponse struct {
	Values []boardAPIResponse `json:"values"`
}

// boardAPIResponse represents a single board in the Jira Agile API response.
type boardAPIResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// sprintListResponse represents the Jira Agile API sprint list response.
type sprintListResponse struct {
	Values []sprintAPIObj `json:"values"`
}

// sprintListItem is retained as an alias for backwards reference.
type sprintListItem = sprintAPIObj

// sprintAPIObj represents a single sprint as returned by the Jira Agile API.
type sprintAPIObj struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	OriginBoardID *int    `json:"originBoardId,omitempty"`
	Goal          string  `json:"goal,omitempty"`
	StartDate     *string `json:"startDate,omitempty"`
	EndDate       *string `json:"endDate,omitempty"`
	ActivatedDate *string `json:"activatedDate,omitempty"`
	CompleteDate  *string `json:"completeDate,omitempty"`
}

// toSprint converts the API response into a domain Sprint.
func (a sprintAPIObj) toSprint() *jira4claude.Sprint {
	return &jira4claude.Sprint{
		ID:            a.ID,
		Name:          a.Name,
		State:         strings.ToLower(a.State),
		OriginBoardID: a.OriginBoardID,
		Goal:          a.Goal,
		StartDate:     parseSprintTime(a.StartDate),
		EndDate:       parseSprintTime(a.EndDate),
		ActivatedDate: parseSprintTime(a.ActivatedDate),
		CompleteDate:  parseSprintTime(a.CompleteDate),
	}
}

// parseSprintTime parses a Jira-formatted date string into a *time.Time.
// Returns nil for nil or unparseable input.
func parseSprintTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000-0700"} {
		if t, err := time.Parse(layout, *s); err == nil {
			return &t
		}
	}
	return nil
}

// Get returns metadata for a sprint by ID, including its issue count.
func (s *SprintService) Get(ctx context.Context, sprintID int) (*jira4claude.Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var resp sprintAPIObj
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to parse response", Inner: err}
	}
	sprint := resp.toSprint()
	count, err := s.IssueCount(ctx, sprintID)
	if err == nil {
		sprint.IssueCount = &count
	}
	return sprint, nil
}

// Create creates a new sprint on the specified board.
func (s *SprintService) Create(ctx context.Context, in jira4claude.SprintCreate) (*jira4claude.Sprint, error) {
	payload := map[string]interface{}{
		"name":          in.Name,
		"originBoardId": in.OriginBoardID,
	}
	if in.StartDate != nil {
		payload["startDate"] = in.StartDate.UTC().Format(time.RFC3339)
	}
	if in.EndDate != nil {
		payload["endDate"] = in.EndDate.UTC().Format(time.RFC3339)
	}
	if in.Goal != "" {
		payload["goal"] = in.Goal
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to marshal request", Inner: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/rest/agile/1.0/sprint", bytes.NewReader(buf))
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	req.Header.Set("Content-Type", "application/json")
	body, err := s.client.DoRequest(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	var resp sprintAPIObj
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to parse response", Inner: err}
	}
	return resp.toSprint(), nil
}

// Update applies a partial update to an existing sprint.
func (s *SprintService) Update(ctx context.Context, sprintID int, in jira4claude.SprintUpdate) (*jira4claude.Sprint, error) {
	payload := map[string]interface{}{}
	if in.Name != "" {
		payload["name"] = in.Name
	}
	if in.State != "" {
		payload["state"] = in.State
	}
	if in.StartDate != nil {
		payload["startDate"] = in.StartDate.UTC().Format(time.RFC3339)
	}
	if in.EndDate != nil {
		payload["endDate"] = in.EndDate.UTC().Format(time.RFC3339)
	}
	if in.Goal != "" {
		payload["goal"] = in.Goal
	}
	if len(payload) == 0 {
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "no fields to update"}
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to marshal request", Inner: err}
	}
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(buf))
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	req.Header.Set("Content-Type", "application/json")
	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var resp sprintAPIObj
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to parse response", Inner: err}
	}
	return resp.toSprint(), nil
}

// Delete removes a sprint.
func (s *SprintService) Delete(ctx context.Context, sprintID int) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	if _, err := s.client.DoRequest(req, http.StatusNoContent); err != nil {
		return err
	}
	return nil
}

// IssueCount returns the number of issues currently in a sprint.
func (s *SprintService) IssueCount(ctx context.Context, sprintID int) (int, error) {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue?maxResults=1&fields=summary", sprintID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to parse response", Inner: err}
	}
	return resp.Total, nil
}
