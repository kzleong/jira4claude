package mock

import (
	"context"

	"github.com/kzleong/jira4claude"
)

// Compile-time interface verifications.
var _ jira4claude.BoardService = (*BoardService)(nil)
var _ jira4claude.SprintService = (*SprintService)(nil)

// BoardService is a mock implementation of jira4claude.BoardService.
// Calling ListBoards without setting ListBoardsFn will panic.
type BoardService struct {
	ListBoardsFn func(ctx context.Context, project string) ([]*jira4claude.Board, error)
}

func (s *BoardService) ListBoards(ctx context.Context, project string) ([]*jira4claude.Board, error) {
	return s.ListBoardsFn(ctx, project)
}

// SprintService is a mock implementation of jira4claude.SprintService.
// Calling a method without setting the corresponding Fn will panic.
type SprintService struct {
	ListSprintsFn func(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error)
	GetFn         func(ctx context.Context, sprintID int) (*jira4claude.Sprint, error)
	CreateFn      func(ctx context.Context, in jira4claude.SprintCreate) (*jira4claude.Sprint, error)
	UpdateFn      func(ctx context.Context, sprintID int, in jira4claude.SprintUpdate) (*jira4claude.Sprint, error)
	DeleteFn      func(ctx context.Context, sprintID int) error
	IssueCountFn  func(ctx context.Context, sprintID int) (int, error)
}

func (s *SprintService) ListSprints(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error) {
	return s.ListSprintsFn(ctx, boardID, states)
}

func (s *SprintService) Get(ctx context.Context, sprintID int) (*jira4claude.Sprint, error) {
	return s.GetFn(ctx, sprintID)
}

func (s *SprintService) Create(ctx context.Context, in jira4claude.SprintCreate) (*jira4claude.Sprint, error) {
	return s.CreateFn(ctx, in)
}

func (s *SprintService) Update(ctx context.Context, sprintID int, in jira4claude.SprintUpdate) (*jira4claude.Sprint, error) {
	return s.UpdateFn(ctx, sprintID, in)
}

func (s *SprintService) Delete(ctx context.Context, sprintID int) error {
	return s.DeleteFn(ctx, sprintID)
}

func (s *SprintService) IssueCount(ctx context.Context, sprintID int) (int, error) {
	return s.IssueCountFn(ctx, sprintID)
}
