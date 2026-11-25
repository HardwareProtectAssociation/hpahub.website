package main

import (
	"encoding/json"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type publicProject struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	Difficulty     string `json:"difficulty"`
	AdventureStyle string `json:"adventureStyle"`
	OwnerID        string `json:"ownerId"`
	OwnerName      string `json:"ownerName"`
	PublishedAt    string `json:"publishedAt"`
	Updated        string `json:"updated"`
}

type publicProjectStage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
	Current     bool   `json:"current"`
}

type publicProjectMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Duties string `json:"duties"`
}

type publicProjectDetail struct {
	publicProject
	Description           string                `json:"description"`
	PlannedStartDate      string                `json:"plannedStartDate"`
	EstimatedDurationDays int                   `json:"estimatedDurationDays"`
	CurrentStage          string                `json:"currentStage"`
	Stages                []publicProjectStage  `json:"stages"`
	Members               []publicProjectMember `json:"members"`
}

type managedProject struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Difficulty   string `json:"difficulty"`
	ReviewStatus string `json:"reviewStatus"`
	OwnerName    string `json:"ownerName"`
	Updated      string `json:"updated"`
}

// bindPublicProjectRoutes exposes only approved, non-deleted projects. Draft,
// pending and rejected projects stay private.
func bindPublicProjectRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/projects", func(e *core.RequestEvent) error {
		noCache(e)
		rows, err := e.App.FindRecordsByFilter("projects", `review_status = "approved"`, "-published_at,-updated", 100, 0)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		owners := map[string]*core.Record{}
		items := make([]publicProject, 0, len(rows))
		for _, project := range rows {
			if project.GetString("deleted_at") != "" {
				continue
			}
			items = append(items, projectSummary(e, project, owners))
		}
		return e.JSON(200, map[string]any{"items": items})
	})
	e.Router.GET("/api/hpahub/projects/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		owners := map[string]*core.Record{}
		detail := publicProjectDetail{
			publicProject:         projectSummary(e, project, owners),
			Description:           projectDescriptionText(project),
			PlannedStartDate:      project.GetString("planned_start_date"),
			EstimatedDurationDays: project.GetInt("estimated_duration_days"),
			Stages:                []publicProjectStage{},
			Members:               []publicProjectMember{},
		}
		stages, err := e.App.FindRecordsByFilter("project_stages", "project_id = {:project}", "sort_order", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load project stages.", err)
		}
		currentStageID := project.GetString("current_stage_id")
		for _, stage := range stages {
			current := stage.Id == currentStageID
			if current {
				detail.CurrentStage = stage.GetString("name")
			}
			detail.Stages = append(detail.Stages, publicProjectStage{
				ID:          stage.Id,
				Name:        stage.GetString("name"),
				Description: stage.GetString("description"),
				SortOrder:   stage.GetInt("sort_order"),
				StartedAt:   stage.GetString("started_at"),
				CompletedAt: stage.GetString("completed_at"),
				Current:     current,
			})
		}
		members, err := e.App.FindRecordsByFilter("project_members", `project_id = {:project} && status = "active"`, "joined_at", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load project members.", err)
		}
		for _, membership := range members {
			member, err := e.App.FindRecordById("users", membership.GetString("user_id"))
			if err != nil {
				continue
			}
			detail.Members = append(detail.Members, publicProjectMember{
				ID:     member.Id,
				Name:   publicPersonName(member),
				Duties: membership.GetString("duties"),
			})
		}
		return e.JSON(200, detail)
	})
	e.Router.GET("/api/hpahub/manage/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		rows, err := e.App.FindRecordsByFilter("projects", `deleted_at = ""`, "-updated", 500, 0)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		owners := map[string]*core.Record{}
		items := make([]managedProject, 0, len(rows))
		for _, project := range rows {
			ownerID := project.GetString("owner_user_id")
			owner := owners[ownerID]
			if owner == nil {
				owner, _ = e.App.FindRecordById("users", ownerID)
				owners[ownerID] = owner
			}
			ownerName := ""
			if owner != nil {
				ownerName = publicPersonName(owner)
			}
			items = append(items, managedProject{
				ID:           project.Id,
				Title:        project.GetString("title"),
				Status:       project.GetString("execution_status"),
				Difficulty:   project.GetString("difficulty"),
				ReviewStatus: project.GetString("review_status"),
				OwnerName:    ownerName,
				Updated:      project.GetString("updated"),
			})
		}
		return e.JSON(200, map[string]any{"items": items})
	})
	e.Router.PATCH("/api/hpahub/manage/projects/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			Difficulty string `json:"difficulty"`
		}
		if err := e.BindBody(&input); err != nil || !validProjectDifficulty(input.Difficulty) {
			return e.BadRequestError("Invalid difficulty.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		project.Set("difficulty", input.Difficulty)
		if err := e.App.Save(project); err != nil {
			return e.InternalServerError("Unable to save difficulty.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "difficulty": input.Difficulty})
	})
}

func validProjectDifficulty(value string) bool {
	for _, option := range []string{"D", "C", "B", "A", "S"} {
		if value == option {
			return true
		}
	}
	return false
}

func projectSummary(e *core.RequestEvent, project *core.Record, owners map[string]*core.Record) publicProject {
	ownerID := project.GetString("owner_user_id")
	owner := owners[ownerID]
	if owner == nil {
		owner, _ = e.App.FindRecordById("users", ownerID)
		owners[ownerID] = owner
	}
	ownerName := ""
	if owner != nil {
		ownerName = publicPersonName(owner)
	}
	return publicProject{
		ID:             project.Id,
		Title:          project.GetString("title"),
		Summary:        project.GetString("summary"),
		Status:         project.GetString("execution_status"),
		Difficulty:     project.GetString("difficulty"),
		AdventureStyle: project.GetString("adventure_style"),
		OwnerID:        ownerID,
		OwnerName:      ownerName,
		PublishedAt:    project.GetString("published_at"),
		Updated:        project.GetString("updated"),
	}
}

func publicPersonName(record *core.Record) string {
	name := strings.TrimSpace(record.GetString("display_name"))
	if name == "" {
		name = strings.TrimSpace(record.GetString("name"))
	}
	return name
}

func projectDescriptionText(project *core.Record) string {
	var data map[string]any
	switch value := project.Get("description").(type) {
	case types.JSONRaw:
		_ = json.Unmarshal(value, &data)
	case string:
		_ = json.Unmarshal([]byte(value), &data)
	case map[string]any:
		data = value
	}
	if text, ok := data["text"].(string); ok {
		return text
	}
	return ""
}
