package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kzleong/jira4claude"
)

// IssueService implements jira4claude.IssueService using the Jira REST API.
type IssueService struct {
	client *Client
}

// issuePath builds an escaped URL path for issue API endpoints.
func issuePath(key string, segments ...string) string {
	path := "/rest/api/3/issue/" + url.PathEscape(key)
	for _, seg := range segments {
		path += "/" + url.PathEscape(seg)
	}
	return path
}

// Compile-time interface verification.
var _ jira4claude.IssueService = (*IssueService)(nil)

// NewIssueService creates a new IssueService using the provided HTTP client.
func NewIssueService(client *Client) *IssueService {
	return &IssueService{client: client}
}

// Create creates a new issue and returns it with Key populated.
func (s *IssueService) Create(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
	reqBody := createRequest{
		Fields: createFields{
			Project:   projectRef{Key: issue.Project},
			Summary:   issue.Summary,
			IssueType: issueTypeRef{Name: issue.Type},
		},
	}

	if issue.Description != nil {
		reqBody.Fields.Description = issue.Description
	}
	if issue.Priority != "" {
		reqBody.Fields.Priority = &priorityRef{Name: issue.Priority}
	}
	if len(issue.Labels) > 0 {
		reqBody.Fields.Labels = issue.Labels
	}
	if issue.Parent != nil {
		reqBody.Fields.Parent = &parentRef{Key: issue.Parent.Key}
	}
	if len(issue.Components) > 0 {
		comps := make([]componentRef, len(issue.Components))
		for i, c := range issue.Components {
			comps[i] = componentRef{Name: c}
		}
		reqBody.Fields.Components = comps
	}
	if issue.StoryPoints != nil {
		reqBody.Fields.StoryPoints = issue.StoryPoints
	}
	if issue.Sprint != nil {
		reqBody.Fields.Sprint = &issue.Sprint.ID
	}
	if issue.EpicKey != "" {
		fieldID, err := s.epicLinkFieldID(ctx)
		if err != nil {
			return nil, err
		}
		reqBody.Fields.CustomFields = map[string]any{fieldID: issue.EpicKey}
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, "/rest/api/3/issue", reqBody)
	if err != nil {
		return nil, err
	}

	respBody, err := s.client.DoRequest(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	var createResp createIssueResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	result := *issue
	result.Key = createResp.Key
	return &result, nil
}

// Get retrieves an issue by its key.
func (s *IssueService) Get(ctx context.Context, key string) (*jira4claude.Issue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key), nil)
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

	issue, err := parseIssueResponse(body)
	if err != nil {
		return nil, err
	}

	return issue, nil
}

// List returns issues matching the filter criteria.
func (s *IssueService) List(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
	jql := filter.JQL
	if jql == "" {
		jql = buildJQL(filter)
	}

	fields := "key,summary,status,issuetype,project,priority,assignee,reporter,labels,issuelinks,parent,created,updated,description,components,customfield_10006,customfield_10001"
	reqURL := "/rest/api/3/search/jql?jql=" + url.QueryEscape(jql) + "&fields=" + fields
	if filter.Limit > 0 {
		reqURL += "&maxResults=" + strconv.Itoa(filter.Limit)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var searchResp searchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	issues := make([]*jira4claude.Issue, 0, len(searchResp.Issues))
	for _, raw := range searchResp.Issues {
		issue, err := parseIssueResponse(raw)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}

	return issues, nil
}

// buildJQL constructs a JQL query from IssueFilter fields.
func buildJQL(filter jira4claude.IssueFilter) string {
	clauses := make([]string, 0, 5+len(filter.Labels))

	if filter.Project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", filter.Project))
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %q", filter.Status))
	} else if filter.ExcludeStatus != "" {
		clauses = append(clauses, fmt.Sprintf("status != %q", filter.ExcludeStatus))
	}
	if filter.Assignee != "" {
		clauses = append(clauses, fmt.Sprintf("assignee = %q", filter.Assignee))
	}
	if filter.Parent != "" {
		clauses = append(clauses, fmt.Sprintf("parent = %q", filter.Parent))
	}
	for _, label := range filter.Labels {
		clauses = append(clauses, fmt.Sprintf("labels = %q", label))
	}

	jql := strings.Join(clauses, " AND ")

	if filter.OrderBy != "" {
		jql += " ORDER BY " + filter.OrderBy
	}

	return jql
}

// Update modifies an existing issue and returns the updated issue.
func (s *IssueService) Update(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
	reqBody := updateRequest{}

	if update.Summary != nil {
		reqBody.Fields.Summary = update.Summary
	}
	if update.Description != nil {
		reqBody.Fields.Description = *update.Description
	}
	if update.Priority != nil {
		reqBody.Fields.Priority = &priorityRef{Name: *update.Priority}
	}
	if update.Assignee != nil {
		if *update.Assignee == "" {
			reqBody.Fields.Assignee = &assigneeField{AccountID: nil}
		} else {
			reqBody.Fields.Assignee = &assigneeField{AccountID: update.Assignee}
		}
	}
	if update.Labels != nil {
		reqBody.Fields.Labels = update.Labels
	}
	if update.Parent != nil {
		if *update.Parent == "" {
			reqBody.Fields.Parent = &parentField{Key: nil}
		} else {
			reqBody.Fields.Parent = &parentField{Key: update.Parent}
		}
	}
	if update.Components != nil {
		comps := make([]componentRef, len(*update.Components))
		for i, c := range *update.Components {
			comps[i] = componentRef{Name: c}
		}
		reqBody.Fields.Components = &comps
	}
	if update.StoryPoints != nil {
		reqBody.Fields.StoryPoints = update.StoryPoints
	}
	if update.Sprint != nil {
		if *update.Sprint == 0 {
			reqBody.Fields.Sprint = &sprintField{ID: nil}
		} else {
			reqBody.Fields.Sprint = &sprintField{ID: update.Sprint}
		}
	}
	if update.Type != nil {
		reqBody.Fields.IssueType = &issueTypeRef{Name: *update.Type}
	}
	if update.Epic != nil {
		fieldID, err := s.epicLinkFieldID(ctx)
		if err != nil {
			return nil, err
		}
		if reqBody.Fields.CustomFields == nil {
			reqBody.Fields.CustomFields = make(map[string]any)
		}
		if *update.Epic == "" {
			reqBody.Fields.CustomFields[fieldID] = nil
		} else {
			reqBody.Fields.CustomFields[fieldID] = *update.Epic
		}
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPut, issuePath(key), reqBody)
	if err != nil {
		return nil, err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	if err != nil {
		return nil, err
	}

	return s.Get(ctx, key)
}

// Delete deletes an issue by its key.
func (s *IssueService) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, issuePath(key), nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// AddComment adds a comment to an issue.
func (s *IssueService) AddComment(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
	reqBody := map[string]any{
		"body": body,
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, issuePath(key, "comment"), reqBody)
	if err != nil {
		return nil, err
	}

	respBody, err := s.client.DoRequest(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	return parseCommentResponse(respBody)
}

// DeleteComment deletes a comment from an issue.
func (s *IssueService) DeleteComment(ctx context.Context, key, commentID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, issuePath(key, "comment", commentID), nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// Transitions returns available workflow transitions for an issue.
func (s *IssueService) Transitions(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key, "transitions"), nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var transitionsResp transitionsResponse
	if err := json.Unmarshal(respBody, &transitionsResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return mapTransitions(transitionsResp.Transitions), nil
}

// Transition moves an issue to a new status.
func (s *IssueService) Transition(ctx context.Context, key, transitionID string) error {
	reqBody := map[string]any{
		"transition": map[string]any{
			"id": transitionID,
		},
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, issuePath(key, "transitions"), reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// Assign assigns an issue to a user by account ID.
// If accountID is empty, the issue is unassigned.
func (s *IssueService) Assign(ctx context.Context, key, accountID string) error {
	var reqBody map[string]any
	if accountID == "" {
		reqBody = map[string]any{"accountId": nil}
	} else {
		reqBody = map[string]any{"accountId": accountID}
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPut, issuePath(key, "assignee"), reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// epicLinkFieldID discovers the Jira custom field ID for "Epic Link" at runtime.
// The field ID varies between Jira instances (e.g. "customfield_10002").
func (s *IssueService) epicLinkFieldID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/field", nil)
	if err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create field list request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return "", err
	}

	var fields []fieldResponse
	if err := json.Unmarshal(body, &fields); err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse field list response",
			Inner:   err,
		}
	}

	for _, f := range fields {
		if f.Name == "Epic Link" {
			return f.ID, nil
		}
	}

	return "", &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: "Epic Link custom field not found; this Jira instance may not support Epic Link",
	}
}

// fieldResponse represents a field entry from the Jira field list API.
type fieldResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// issueResponse represents the JSON structure returned by Jira API for an issue.
type issueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Project     struct{ Key string }  `json:"project"`
		Summary     string                `json:"summary"`
		Description *jira4claude.ADFNode  `json:"description"`
		Status      struct{ Name string } `json:"status"`
		IssueType   struct{ Name string } `json:"issuetype"`
		Priority    struct{ Name string } `json:"priority"`
		Assignee    *userResponse         `json:"assignee"`
		Reporter    *userResponse         `json:"reporter"`
		Labels      []string              `json:"labels"`
		IssueLinks  []issueLinkResponse   `json:"issuelinks"`
		Subtasks    []linkedIssueResponse `json:"subtasks"`
		Comment     *commentsResponse     `json:"comment"`
		Parent      *linkedIssueResponse  `json:"parent"`
		Components  []componentResponse   `json:"components"`
		StoryPoints *float64              `json:"customfield_10006"`
		Sprint      sprintAPIList         `json:"customfield_10001"`
		Created     string                `json:"created"`
		Updated     string                `json:"updated"`
	} `json:"fields"`
}

// componentResponse represents a component in the Jira API response.
type componentResponse struct {
	Name string `json:"name"`
}

// sprintAPIResponse represents a sprint entry in the Jira API response.
type sprintAPIResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// sprintAPIList handles two Jira Server response formats for customfield_10001:
//   - Array of objects: [{"id":44182,"name":"Sprint 7","state":"active"}]
//   - Legacy GreenHopper string: "Sprint@...[id=44182,state=active,name=Sprint 7,...]"
type sprintAPIList []sprintAPIResponse

// UnmarshalJSON implements json.Unmarshaler for sprintAPIList.
func (s *sprintAPIList) UnmarshalJSON(data []byte) error {
	var arr []sprintAPIResponse
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	var strs []string
	if err := json.Unmarshal(data, &strs); err == nil {
		for _, str := range strs {
			if sp := parseSprintString(str); sp != nil {
				*s = append(*s, *sp)
			}
		}
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return nil // null or unknown format — ignore silently
	}
	if sp := parseSprintString(str); sp != nil {
		*s = []sprintAPIResponse{*sp}
	}
	return nil
}

// parseSprintString parses the legacy GreenHopper sprint string format:
// "Sprint@...[id=44182,rapidViewId=7261,state=active,name=My Sprint,...]"
func parseSprintString(s string) *sprintAPIResponse {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return nil
	}
	content := s[start+1 : end]
	result := &sprintAPIResponse{}
	for _, part := range strings.Split(content, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "id":
			id, err := strconv.Atoi(val)
			if err == nil {
				result.ID = id
			}
		case "state":
			result.State = val
		case "name":
			result.Name = val
		}
	}
	if result.ID == 0 {
		return nil
	}
	return result
}

// commentsResponse represents the comment container in the Jira API response.
type commentsResponse struct {
	Comments []commentAPIResponse `json:"comments"`
	Total    int                  `json:"total"`
}

// commentAPIResponse represents a single comment in the issue response.
type commentAPIResponse struct {
	ID      string               `json:"id"`
	Author  *userResponse        `json:"author"`
	Body    *jira4claude.ADFNode `json:"body"`
	Created string               `json:"created"`
}

// issueLinkResponse represents a link in the Jira API response.
type issueLinkResponse struct {
	ID   string `json:"id"`
	Type struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	OutwardIssue *linkedIssueResponse `json:"outwardIssue"`
	InwardIssue  *linkedIssueResponse `json:"inwardIssue"`
}

// linkedIssueResponse represents a linked issue in the Jira API response.
type linkedIssueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary   string                `json:"summary"`
		Status    struct{ Name string } `json:"status"`
		IssueType struct{ Name string } `json:"issuetype"`
	} `json:"fields"`
}

// userResponse mirrors the Jira Cloud / Data Center 9+ user JSON.
// Users are identified by accountId.
type userResponse struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// transitionResponse represents a single transition in the Jira API response.
type transitionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// createIssueResponse represents the JSON structure returned by Jira API when creating an issue.
type createIssueResponse struct {
	Key string `json:"key"`
}

// searchResponse represents the JSON structure returned by Jira API for issue search.
type searchResponse struct {
	Issues []json.RawMessage `json:"issues"`
}

// transitionsResponse represents the JSON structure returned by Jira API for transitions.
type transitionsResponse struct {
	Transitions []transitionResponse `json:"transitions"`
}

// findLinkIssueResponse represents the minimal issue structure for finding links.
type findLinkIssueResponse struct {
	Fields struct {
		IssueLinks []findLinkIssueLinkResponse `json:"issuelinks"`
	} `json:"fields"`
}

// findLinkIssueLinkResponse represents a link entry when searching for a specific link.
type findLinkIssueLinkResponse struct {
	ID           string                  `json:"id"`
	OutwardIssue *findLinkLinkedResponse `json:"outwardIssue"`
	InwardIssue  *findLinkLinkedResponse `json:"inwardIssue"`
}

// findLinkLinkedResponse represents a linked issue key when searching for a specific link.
type findLinkLinkedResponse struct {
	Key string `json:"key"`
}

// parseIssueResponse parses the JSON response from Jira into a domain Issue.
func parseIssueResponse(body []byte) (*jira4claude.Issue, error) {
	var resp issueResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	issue := &jira4claude.Issue{
		Key:         resp.Key,
		Project:     resp.Fields.Project.Key,
		Summary:     resp.Fields.Summary,
		Description: resp.Fields.Description,
		Status:      resp.Fields.Status.Name,
		Type:        resp.Fields.IssueType.Name,
		Priority:    resp.Fields.Priority.Name,
		Labels:      resp.Fields.Labels,
		StoryPoints: resp.Fields.StoryPoints,
	}

	if len(resp.Fields.Components) > 0 {
		issue.Components = make([]string, len(resp.Fields.Components))
		for i, c := range resp.Fields.Components {
			issue.Components[i] = c.Name
		}
	}

	if len(resp.Fields.Sprint) > 0 {
		chosen := resp.Fields.Sprint[0]
		for _, s := range resp.Fields.Sprint {
			if s.State == "active" {
				chosen = s
				break
			}
		}
		issue.Sprint = &jira4claude.Sprint{
			ID:    chosen.ID,
			Name:  chosen.Name,
			State: chosen.State,
		}
	}

	issue.Parent = mapLinkedIssue(resp.Fields.Parent)
	issue.Assignee = mapUser(resp.Fields.Assignee)
	issue.Reporter = mapUser(resp.Fields.Reporter)

	if resp.Fields.Created != "" {
		if t, err := parseJiraTime(resp.Fields.Created); err == nil {
			issue.Created = t
		}
	}
	if resp.Fields.Updated != "" {
		if t, err := parseJiraTime(resp.Fields.Updated); err == nil {
			issue.Updated = t
		}
	}

	issue.Links = mapIssueLinks(resp.Fields.IssueLinks)
	issue.Subtasks = mapSubtasks(resp.Fields.Subtasks)
	issue.Comments = mapComments(resp.Fields.Comment)

	return issue, nil
}

// mapUser converts a userResponse to a domain User. Returns nil if input is nil.
func mapUser(resp *userResponse) *jira4claude.User {
	if resp == nil {
		return nil
	}
	return &jira4claude.User{
		AccountID:   resp.AccountID,
		DisplayName: resp.DisplayName,
		Email:       resp.EmailAddress,
	}
}

// mapLinkedIssue converts a linkedIssueResponse to a domain LinkedIssue. Returns nil if input is nil.
func mapLinkedIssue(resp *linkedIssueResponse) *jira4claude.LinkedIssue {
	if resp == nil {
		return nil
	}
	return &jira4claude.LinkedIssue{
		Key:     resp.Key,
		Summary: resp.Fields.Summary,
		Status:  resp.Fields.Status.Name,
		Type:    resp.Fields.IssueType.Name,
	}
}

// mapIssueLinks converts a slice of issueLinkResponse to domain IssueLinks.
func mapIssueLinks(links []issueLinkResponse) []*jira4claude.IssueLink {
	if len(links) == 0 {
		return nil
	}
	result := make([]*jira4claude.IssueLink, len(links))
	for i, link := range links {
		result[i] = &jira4claude.IssueLink{
			ID: link.ID,
			Type: jira4claude.IssueLinkType{
				Name:    link.Type.Name,
				Inward:  link.Type.Inward,
				Outward: link.Type.Outward,
			},
			OutwardIssue: mapLinkedIssue(link.OutwardIssue),
			InwardIssue:  mapLinkedIssue(link.InwardIssue),
		}
	}
	return result
}

// mapSubtasks converts a slice of linkedIssueResponse to domain LinkedIssues for subtasks.
func mapSubtasks(subtasks []linkedIssueResponse) []*jira4claude.LinkedIssue {
	if len(subtasks) == 0 {
		return nil
	}
	result := make([]*jira4claude.LinkedIssue, len(subtasks))
	for i := range subtasks {
		result[i] = mapLinkedIssue(&subtasks[i])
	}
	return result
}

// mapComments converts a commentsResponse to domain Comments.
func mapComments(resp *commentsResponse) []*jira4claude.Comment {
	if resp == nil || len(resp.Comments) == 0 {
		return nil
	}
	result := make([]*jira4claude.Comment, len(resp.Comments))
	for i, c := range resp.Comments {
		comment := &jira4claude.Comment{
			ID:     c.ID,
			Body:   c.Body,
			Author: mapUser(c.Author),
		}
		if c.Created != "" {
			if t, err := parseJiraTime(c.Created); err == nil {
				comment.Created = t
			}
		}
		result[i] = comment
	}
	return result
}

// mapTransitions converts a slice of transitionResponse to domain Transitions.
func mapTransitions(transitions []transitionResponse) []*jira4claude.Transition {
	if len(transitions) == 0 {
		return nil
	}
	result := make([]*jira4claude.Transition, len(transitions))
	for i, t := range transitions {
		result[i] = &jira4claude.Transition{
			ID:   t.ID,
			Name: t.Name,
		}
	}
	return result
}

// parseJiraTime parses a Jira timestamp string.
func parseJiraTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000-0700", s)
}

// Link creates a link between two issues.
func (s *IssueService) Link(ctx context.Context, inwardKey, linkType, outwardKey string) error {
	reqBody := map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": inwardKey},
		"outwardIssue": map[string]any{"key": outwardKey},
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, "/rest/api/3/issueLink", reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusCreated)
	return err
}

// Unlink removes a link between two issues.
func (s *IssueService) Unlink(ctx context.Context, key1, key2 string) error {
	linkID, err := s.findLinkID(ctx, key1, key2)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "/rest/api/3/issueLink/"+url.PathEscape(linkID), nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// findLinkID finds the link ID connecting two issues.
func (s *IssueService) findLinkID(ctx context.Context, key1, key2 string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key1), nil)
	if err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return "", err
	}

	var issueResp findLinkIssueResponse
	if err := json.Unmarshal(respBody, &issueResp); err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	for _, link := range issueResp.Fields.IssueLinks {
		if link.OutwardIssue != nil && link.OutwardIssue.Key == key2 {
			return link.ID, nil
		}
		if link.InwardIssue != nil && link.InwardIssue.Key == key2 {
			return link.ID, nil
		}
	}

	return "", &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: fmt.Sprintf("no link found between %s and %s", key1, key2),
	}
}

// commentResponse represents the JSON structure returned by Jira API for a comment.
type commentResponse struct {
	ID      string               `json:"id"`
	Author  *userResponse        `json:"author"`
	Body    *jira4claude.ADFNode `json:"body"`
	Created string               `json:"created"`
}

// parseCommentResponse parses the JSON response from Jira into a domain Comment.
func parseCommentResponse(body []byte) (*jira4claude.Comment, error) {
	var resp commentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	comment := &jira4claude.Comment{
		ID:   resp.ID,
		Body: resp.Body,
	}

	if resp.Author != nil {
		comment.Author = &jira4claude.User{
			AccountID:   resp.Author.AccountID,
			DisplayName: resp.Author.DisplayName,
			Email:       resp.Author.EmailAddress,
		}
	}

	if resp.Created != "" {
		if t, err := parseJiraTime(resp.Created); err == nil {
			comment.Created = t
		}
	}

	return comment, nil
}
