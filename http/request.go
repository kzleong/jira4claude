package http

import "encoding/json"

// createRequest represents the request body for creating a Jira issue.
type createRequest struct {
	Fields createFields `json:"fields"`
}

// createFields contains the fields for creating an issue.
// CustomFields holds dynamic fields (e.g., the Epic Link custom field whose ID
// varies per Jira instance) and are merged into the JSON output by MarshalJSON.
type createFields struct {
	Project      projectRef     `json:"project"`
	Summary      string         `json:"summary"`
	IssueType    issueTypeRef   `json:"issuetype"`
	Description  any            `json:"description,omitempty"`
	Priority     *priorityRef   `json:"priority,omitempty"`
	Labels       []string       `json:"labels,omitempty"`
	Parent       *parentRef     `json:"parent,omitempty"`
	Components   []componentRef `json:"components,omitempty"`
	StoryPoints  *float64       `json:"customfield_10006,omitempty"`
	Sprint       *int           `json:"customfield_10001,omitempty"`
	CustomFields map[string]any `json:"-"`
}

// MarshalJSON merges CustomFields into the standard fields JSON output.
func (f createFields) MarshalJSON() ([]byte, error) {
	type alias createFields
	data, err := json.Marshal(alias(f))
	if err != nil {
		return nil, err
	}
	if len(f.CustomFields) == 0 {
		return data, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for k, v := range f.CustomFields {
		m[k] = v
	}
	return json.Marshal(m)
}

// projectRef identifies a project by key.
type projectRef struct {
	Key string `json:"key"`
}

// issueTypeRef identifies an issue type by name.
type issueTypeRef struct {
	Name string `json:"name"`
}

// priorityRef identifies a priority by name.
type priorityRef struct {
	Name string `json:"name"`
}

// parentRef identifies a parent issue by key.
type parentRef struct {
	Key string `json:"key"`
}

// componentRef identifies a component by name.
type componentRef struct {
	Name string `json:"name"`
}

// sprintField handles the sprint custom field (customfield_10001) for updates.
// It marshals to JSON null when ID is nil (to clear the sprint) or to the integer
// sprint ID otherwise.
type sprintField struct {
	ID *int
}

// MarshalJSON implements json.Marshaler for sprintField.
func (s sprintField) MarshalJSON() ([]byte, error) {
	if s.ID == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*s.ID)
}

// updateRequest represents the request body for updating a Jira issue.
type updateRequest struct {
	Fields updateFields `json:"fields"`
}

// updateFields contains the fields for updating an issue.
// All fields are optional - only set fields will be sent.
// CustomFields holds dynamic fields (e.g., Epic Link) merged via MarshalJSON.
type updateFields struct {
	Summary      *string         `json:"summary,omitempty"`
	Description  any             `json:"description,omitempty"`
	Priority     *priorityRef    `json:"priority,omitempty"`
	Assignee     *assigneeField  `json:"assignee,omitempty"`
	Labels       *[]string       `json:"labels,omitempty"`
	Parent       *parentField    `json:"parent,omitempty"`
	Components   *[]componentRef `json:"components,omitempty"`
	StoryPoints  *float64        `json:"customfield_10006,omitempty"`
	Sprint       *sprintField    `json:"customfield_10001,omitempty"`
	IssueType    *issueTypeRef   `json:"issuetype,omitempty"`
	CustomFields map[string]any  `json:"-"`
}

// MarshalJSON merges CustomFields into the standard fields JSON output.
func (f updateFields) MarshalJSON() ([]byte, error) {
	type alias updateFields
	data, err := json.Marshal(alias(f))
	if err != nil {
		return nil, err
	}
	if len(f.CustomFields) == 0 {
		return data, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for k, v := range f.CustomFields {
		m[k] = v
	}
	return json.Marshal(m)
}

// assigneeRef identifies an assignee by account ID (Jira Cloud / Data Center 9+).
type assigneeRef struct {
	AccountID string `json:"accountId"`
}

// assigneeField wraps an optional assignee value.
// When AccountID is nil, it marshals to JSON null (for unassignment).
// When AccountID is set, it marshals to {"accountId": "..."}.
type assigneeField struct {
	AccountID *string
}

// MarshalJSON implements json.Marshaler for assigneeField.
func (a assigneeField) MarshalJSON() ([]byte, error) {
	if a.AccountID == nil {
		return []byte("null"), nil
	}
	return json.Marshal(assigneeRef{AccountID: *a.AccountID})
}

// parentField wraps an optional parent value for updates.
// When Key is nil, it marshals to JSON null (to clear parent).
// When Key is set, it marshals to {"key": "..."}.
type parentField struct {
	Key *string
}

// MarshalJSON implements json.Marshaler for parentField.
func (p parentField) MarshalJSON() ([]byte, error) {
	if p.Key == nil {
		return []byte("null"), nil
	}
	return json.Marshal(parentRef{Key: *p.Key})
}
