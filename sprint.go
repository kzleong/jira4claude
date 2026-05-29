package jira4claude

import (
	"context"
	"time"
)

// Board represents a Jira board.
type Board struct {
	ID   int
	Name string
	Type string // "scrum" or "kanban"
}

// Sprint represents a Jira sprint.
type Sprint struct {
	ID            int
	Name          string
	State         string // "active", "future", "closed"
	OriginBoardID *int
	Goal          string
	StartDate     *time.Time
	EndDate       *time.Time
	ActivatedDate *time.Time
	CompleteDate  *time.Time
	IssueCount    *int // populated by SprintService.Get when available
}

// SprintCreate describes fields used when creating a new sprint.
type SprintCreate struct {
	Name          string
	OriginBoardID int
	StartDate     *time.Time
	EndDate       *time.Time
	Goal          string
}

// SprintUpdate describes partial updates to an existing sprint.
// Nil pointer fields mean "do not change". For Name and Goal, the empty
// string means "do not change" — use SprintUpdate to clear goal explicitly
// is not currently supported by Jira on this endpoint.
type SprintUpdate struct {
	Name      string
	State     string // "", "active", "closed", "future"
	StartDate *time.Time
	EndDate   *time.Time
	Goal      string
}

// BoardService defines operations for discovering Jira boards.
type BoardService interface {
	// ListBoards returns boards associated with the given project key.
	ListBoards(ctx context.Context, project string) ([]*Board, error)
}

// SprintService defines operations for managing Jira sprints.
type SprintService interface {
	// ListSprints returns sprints on the given board.
	// If states is empty, returns active and future sprints.
	// Common states: "active", "future", "closed".
	ListSprints(ctx context.Context, boardID int, states []string) ([]*Sprint, error)

	// Get returns metadata for a sprint by ID. The returned Sprint
	// includes IssueCount populated.
	Get(ctx context.Context, sprintID int) (*Sprint, error)

	// Create creates a new sprint on the specified board.
	Create(ctx context.Context, in SprintCreate) (*Sprint, error)

	// Update applies a partial update to an existing sprint.
	Update(ctx context.Context, sprintID int, in SprintUpdate) (*Sprint, error)

	// Delete removes a sprint. Jira deletes orphan issues into the backlog.
	Delete(ctx context.Context, sprintID int) error

	// IssueCount returns the number of issues currently in a sprint.
	IssueCount(ctx context.Context, sprintID int) (int, error)
}
