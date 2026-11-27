package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type publicProject struct {
	Redacted       bool   `json:"redacted,omitempty"`
	OwnerRedacted  bool   `json:"ownerRedacted,omitempty"`
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	Difficulty     string `json:"difficulty"`
	AdventureStyle string `json:"adventureStyle"`
	OwnerID        string `json:"ownerId"`
	OwnerName      string `json:"ownerName"`
	OwnerAvatarURL string `json:"ownerAvatarUrl"`
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
	Redacted bool   `json:"redacted,omitempty"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Duties   string `json:"duties"`
}

type publicProjectDetail struct {
	publicProject
	Description           string                `json:"description"`
	PlannedStartDate      string                `json:"plannedStartDate"`
	EstimatedDurationDays int                   `json:"estimatedDurationDays"`
	CurrentStage          string                `json:"currentStage"`
	Stages                []publicProjectStage  `json:"stages"`
	Members               []publicProjectMember `json:"members"`
	CanApply              bool                  `json:"canApply"`
	ApplicationPending    bool                  `json:"applicationPending"`
	CanRequestImprovement bool                  `json:"canRequestImprovement"`
	ImprovementPending    bool                  `json:"improvementPending"`
	CanFollow             bool                  `json:"canFollow"`
	Following             bool                  `json:"following"`
	JoinMode              string                `json:"joinMode"`
	ProposedDifficulty    string                `json:"proposedDifficulty"`
	OwnerConfirmed        bool                  `json:"ownerConfirmed"`
	ApproveCount          int                   `json:"approveCount"`
	OpposeCount           int                   `json:"opposeCount"`
	IsOwner               bool                  `json:"isOwner"`
}

type managedProject struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Summary            string `json:"summary"`
	Status             string `json:"status"`
	Difficulty         string `json:"difficulty"`
	ReviewStatus       string `json:"reviewStatus"`
	OwnerName          string `json:"ownerName"`
	OwnerID            string `json:"ownerId"`
	Created            string `json:"created"`
	Updated            string `json:"updated"`
	ProposedDifficulty string `json:"proposedDifficulty"`
	OwnerConfirmed     bool   `json:"ownerConfirmed"`
	ApproveCount       int    `json:"approveCount"`
	OpposeCount        int    `json:"opposeCount"`
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
			ProposedDifficulty:    project.GetString("owner_proposed_difficulty"),
			OwnerConfirmed:        project.GetBool("owner_difficulty_confirmed"),
			JoinMode:              project.GetString("join_mode"),
		}
		// Compute vote counts
		votes, _ := e.App.FindRecordsByFilter("project_difficulty_votes",
			"project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
		for _, v := range votes {
			if v.GetString("stance") == "approve" {
				detail.ApproveCount++
			} else {
				detail.OpposeCount++
			}
		}
		// Check if viewer is owner
		if e.Auth != nil {
			if owner, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member {
				if project.GetString("owner_user_id") == owner.Id {
					detail.IsOwner = true
				}
			}
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
			if profileSectionHidden(member, "projects") || profileSectionHidden(member, "project:"+project.Id) {
				detail.Members = append(detail.Members, publicProjectMember{Redacted: true, Name: maskedText(publicPersonName(member)), Duties: maskedText(membership.GetString("duties"))})
				continue
			}
			detail.Members = append(detail.Members, publicProjectMember{
				ID: member.Id, Name: publicPersonName(member), Duties: membership.GetString("duties"),
			})
		}
		if e.Auth != nil {
			if viewer, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member && viewer.Id != project.GetString("owner_user_id") {
				detail.CanFollow = true
				if follow, err := e.App.FindFirstRecordByFilter("project_follows", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					detail.Following = follow.GetBool("is_active")
				}
				activeMember := false
				if _, err := e.App.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					activeMember = true
				}
				hasPending := false
				if pending, err := e.App.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					hasPending = true
					if pending.GetString("source") == "improvement" {
						detail.ImprovementPending = true
					} else {
						detail.ApplicationPending = true
					}
				}
				if !activeMember && !hasPending {
					if project.GetString("execution_status") == "completed" {
						detail.CanRequestImprovement = true
					} else if project.GetString("collaboration_mode") != "independent" {
						detail.CanApply = true
					}
				}
			}
		}
		return e.JSON(200, detail)
	})
	e.Router.POST("/api/hpahub/projects/{id}/apply", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load project.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You already own this project.", nil)
		}
		if project.GetString("execution_status") == "completed" {
			return e.BadRequestError("Completed projects accept improvement requests instead.", nil)
		}
		if project.GetString("collaboration_mode") == "independent" {
			return e.BadRequestError("This is an independent project.", nil)
		}

		var input struct {
			Message    string `json:"message"`
			Profession string `json:"profession"`
			Time       string `json:"time"`
			Ability    string `json:"ability"`
		}
		if err := e.BindBody(&input); err != nil || len([]rune(strings.TrimSpace(input.Message))) > 500 {
			return e.BadRequestError("Invalid application.", nil)
		}
		if len([]rune(strings.TrimSpace(input.Profession))) > 100 || len([]rune(strings.TrimSpace(input.Time))) > 100 || len([]rune(strings.TrimSpace(input.Ability))) > 200 {
			return e.BadRequestError("Invalid application.", nil)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
				return e.BadRequestError("You are already a project member.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return e.InternalServerError("Unable to load membership.", err)
			}
			if _, err := tx.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
				return e.BadRequestError("You already have a pending application.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return e.InternalServerError("Unable to load application.", err)
			}
			collection, err := tx.FindCollectionByNameOrId("project_join_requests")
			if err != nil {
				return e.InternalServerError("Unable to submit application.", err)
			}
			request := core.NewRecord(collection)
			request.Set("project_id", project.Id)
			request.Set("user_id", viewer.Id)
			request.Set("source", "application")
			request.Set("message", strings.TrimSpace(input.Message))
			request.Set("apply_profession", strings.TrimSpace(input.Profession))
			request.Set("apply_time", strings.TrimSpace(input.Time))
			request.Set("apply_ability", strings.TrimSpace(input.Ability))
			request.Set("status", "pending")
			if err := tx.Save(request); err != nil {
				return e.InternalServerError("Unable to submit application.", err)
			}
			// Free join mode: auto-accept
			if project.GetString("join_mode") == "open" {
				request.Set("status", "accepted")
				request.Set("handled_at", time.Now())
				if err := tx.Save(request); err != nil {
					return e.InternalServerError("Unable to submit application.", err)
				}
				// Add as project member
				membership, err := tx.FindFirstRecordByFilter("project_members", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id})
				if errors.Is(err, sql.ErrNoRows) {
					mCollection, err := tx.FindCollectionByNameOrId("project_members")
					if err != nil {
						return e.InternalServerError("Unable to add member.", err)
					}
					membership = core.NewRecord(mCollection)
					membership.Set("project_id", project.Id)
					membership.Set("user_id", viewer.Id)
				} else if err != nil {
					return e.InternalServerError("Unable to add member.", err)
				}
				membership.Set("status", "active")
				membership.Set("joined_at", time.Now())
				if err := tx.Save(membership); err != nil {
					return e.InternalServerError("Unable to add member.", err)
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true, "autoAccepted": project.GetString("join_mode") == "open"})
	})
	e.Router.POST("/api/hpahub/projects/{id}/improvement", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load project.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("execution_status") != "completed" {
			return e.BadRequestError("Only completed projects accept improvement requests.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You already own this project.", nil)
		}
		if _, err := e.App.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
			return e.BadRequestError("You are already a project member.", nil)
		}
		if _, err := e.App.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
			return e.BadRequestError("You already have a pending request.", nil)
		}
		var input struct {
			Message string `json:"message"`
		}
		if err := e.BindBody(&input); err != nil || len([]rune(strings.TrimSpace(input.Message))) > 500 {
			return e.BadRequestError("Invalid improvement request.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("project_join_requests")
		if err != nil {
			return e.InternalServerError("Unable to submit improvement request.", err)
		}
		request := core.NewRecord(collection)
		request.Set("project_id", project.Id)
		request.Set("user_id", viewer.Id)
		request.Set("source", "improvement")
		request.Set("message", strings.TrimSpace(input.Message))
		request.Set("status", "pending")
		if err := e.App.Save(request); err != nil {
			return e.InternalServerError("Unable to submit improvement request.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
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
				ID:                 project.Id,
				Title:              project.GetString("title"),
				Summary:            project.GetString("summary"),
				Status:             project.GetString("execution_status"),
				Difficulty:         project.GetString("difficulty"),
				ReviewStatus:       project.GetString("review_status"),
				OwnerName:          ownerName,
				OwnerID:            ownerID,
				Created:            project.GetString("created"),
				Updated:            project.GetString("updated"),
				ProposedDifficulty: project.GetString("owner_proposed_difficulty"),
				OwnerConfirmed:     project.GetBool("owner_difficulty_confirmed"),
				ApproveCount:       0,
				OpposeCount:        0,
			})
			// Compute vote counts
			votes, _ := e.App.FindRecordsByFilter("project_difficulty_votes",
				"project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
			for _, v := range votes {
				if v.GetString("stance") == "approve" {
					items[len(items)-1].ApproveCount++
				} else {
					items[len(items)-1].OpposeCount++
				}
			}
		}
		return e.JSON(200, map[string]any{"items": items})
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
	ownerAvatarURL := ""
	ownerRedacted := false
	if owner != nil {
		ownerName = publicPersonName(owner)
		ownerRedacted = profileSectionHidden(owner, "projects") || profileSectionHidden(owner, "project:"+project.Id)
		if ownerRedacted {
			ownerName = maskedText(ownerName)
			ownerID = ""
		}
		if !ownerRedacted && owner.GetString("avatar") != "" {
			ownerAvatarURL = contentURL("members", owner.Id, "avatar")
		}
	}
	return publicProject{
		OwnerRedacted:  ownerRedacted,
		ID:             project.Id,
		Title:          project.GetString("title"),
		Summary:        project.GetString("summary"),
		Status:         project.GetString("execution_status"),
		Difficulty:     project.GetString("difficulty"),
		AdventureStyle: project.GetString("adventure_style"),
		OwnerID:        ownerID,
		OwnerName:      ownerName,
		OwnerAvatarURL: ownerAvatarURL,
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
