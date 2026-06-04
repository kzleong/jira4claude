package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kzleong/jira4claude"
)

// BoardCmd groups board subcommands.
type BoardCmd struct {
	List BoardListCmd `cmd:"" help:"List boards for a project"`
}

// SprintCmd groups sprint subcommands.
type SprintCmd struct {
	List   SprintListCmd   `cmd:"" help:"List sprints on a board"`
	View   SprintViewCmd   `cmd:"" help:"View sprint metadata and issue count"`
	Create SprintCreateCmd `cmd:"" help:"Create a sprint on a board"`
	Update SprintUpdateCmd `cmd:"" help:"Update a sprint (rename, re-date, start/close, goal)"`
	Delete SprintDeleteCmd `cmd:"" help:"Delete a sprint"`
}

// SprintContext provides dependencies for board and sprint commands.
type SprintContext struct {
	BoardService  jira4claude.BoardService
	SprintService jira4claude.SprintService
	Printer       jira4claude.Printer
	Config        *jira4claude.Config
}

// BoardListCmd lists boards for a project.
type BoardListCmd struct {
	Project string `help:"Project key (defaults to config)" short:"p"`
}

// Run executes the board list command.
func (c *BoardListCmd) Run(ctx *SprintContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	boards, err := ctx.BoardService.ListBoards(context.Background(), project)
	if err != nil {
		return err
	}

	ctx.Printer.Boards(jira4claude.ToBoardViews(boards))
	return nil
}

// SprintListCmd lists sprints on a board.
type SprintListCmd struct {
	Board int    `arg:"" help:"Board ID"`
	State string `help:"Filter by state: active, future, closed (default: active,future)"`
}

// Run executes the sprint list command.
func (c *SprintListCmd) Run(ctx *SprintContext) error {
	var states []string
	if c.State != "" {
		states = strings.Split(c.State, ",")
	}

	sprints, err := ctx.SprintService.ListSprints(context.Background(), c.Board, states)
	if err != nil {
		return err
	}

	ctx.Printer.Sprints(jira4claude.ToSprintViews(sprints))
	return nil
}

// SprintViewCmd views a single sprint's metadata.
type SprintViewCmd struct {
	ID int `arg:"" help:"Sprint ID"`
}

// Run executes the sprint view command.
func (c *SprintViewCmd) Run(ctx *SprintContext) error {
	sprint, err := ctx.SprintService.Get(context.Background(), c.ID)
	if err != nil {
		return err
	}
	ctx.Printer.Sprint(jira4claude.ToSprintView(sprint))
	return nil
}

// SprintCreateCmd creates a new sprint on a board.
type SprintCreateCmd struct {
	Name      string `help:"Sprint name" required:""`
	Board     int    `help:"Origin board ID" required:""`
	StartDate string `help:"Start date (RFC3339, e.g. 2026-06-01T09:00:00Z)" name:"start-date"`
	EndDate   string `help:"End date (RFC3339)" name:"end-date"`
	Goal      string `help:"Sprint goal"`
}

// Run executes the sprint create command.
func (c *SprintCreateCmd) Run(ctx *SprintContext) error {
	in := jira4claude.SprintCreate{
		Name:          c.Name,
		OriginBoardID: c.Board,
		Goal:          c.Goal,
	}
	if c.StartDate != "" {
		t, err := parseSprintDate(c.StartDate)
		if err != nil {
			return fmt.Errorf("invalid --start-date: %w", err)
		}
		in.StartDate = &t
	}
	if c.EndDate != "" {
		t, err := parseSprintDate(c.EndDate)
		if err != nil {
			return fmt.Errorf("invalid --end-date: %w", err)
		}
		in.EndDate = &t
	}
	sprint, err := ctx.SprintService.Create(context.Background(), in)
	if err != nil {
		return err
	}
	ctx.Printer.Success(fmt.Sprintf("Created sprint %d: %s", sprint.ID, sprint.Name))
	ctx.Printer.Sprint(jira4claude.ToSprintView(sprint))
	return nil
}

// SprintUpdateCmd applies a partial update to a sprint.
type SprintUpdateCmd struct {
	ID        int    `arg:"" help:"Sprint ID"`
	Name      string `help:"Rename the sprint"`
	State     string `help:"New state: active, closed, future" enum:",active,closed,future" default:""`
	StartDate string `help:"Start date (RFC3339)" name:"start-date"`
	EndDate   string `help:"End date (RFC3339)" name:"end-date"`
	Goal      string `help:"Sprint goal"`
}

// Run executes the sprint update command.
func (c *SprintUpdateCmd) Run(ctx *SprintContext) error {
	in := jira4claude.SprintUpdate{
		Name:  c.Name,
		State: c.State,
		Goal:  c.Goal,
	}
	if c.StartDate != "" {
		t, err := parseSprintDate(c.StartDate)
		if err != nil {
			return fmt.Errorf("invalid --start-date: %w", err)
		}
		in.StartDate = &t
	}
	if c.EndDate != "" {
		t, err := parseSprintDate(c.EndDate)
		if err != nil {
			return fmt.Errorf("invalid --end-date: %w", err)
		}
		in.EndDate = &t
	}
	sprint, err := ctx.SprintService.Update(context.Background(), c.ID, in)
	if err != nil {
		return err
	}
	ctx.Printer.Success(fmt.Sprintf("Updated sprint %d", sprint.ID))
	ctx.Printer.Sprint(jira4claude.ToSprintView(sprint))
	return nil
}

// SprintDeleteCmd deletes a sprint.
type SprintDeleteCmd struct {
	ID    int  `arg:"" help:"Sprint ID"`
	Force bool `help:"Delete even if the sprint contains issues (issues will be moved to the backlog)"`
}

// Run executes the sprint delete command.
func (c *SprintDeleteCmd) Run(ctx *SprintContext) error {
	count, err := ctx.SprintService.IssueCount(context.Background(), c.ID)
	if err != nil {
		return err
	}
	if count > 0 && !c.Force {
		return fmt.Errorf("sprint %d contains %d issue(s); pass --force to delete anyway (issues will be moved to the backlog)", c.ID, count)
	}
	if err := ctx.SprintService.Delete(context.Background(), c.ID); err != nil {
		return err
	}
	ctx.Printer.Success(fmt.Sprintf("Deleted sprint %d (%d issue(s) freed)", c.ID, count))
	return nil
}

// parseSprintDate accepts RFC3339, RFC3339 without seconds, or date-only inputs.
func parseSprintDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q (use RFC3339 like 2026-06-01T09:00:00Z)", s)
}
