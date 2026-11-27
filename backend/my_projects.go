package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type myProject struct {
	OwnerRedacted  bool   `json:"ownerRedacted,omitempty"`
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	ReviewStatus   string `json:"reviewStatus"`
	Difficulty     string `json:"difficulty"`
	OwnerName      string `json:"ownerName"`
	OwnerAvatarURL string `json:"ownerAvatarUrl"`
	Updated        string `json:"updated"`
}

func bindMyProjectRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/me/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		creator, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Title                    string `json:"title"`
			Summary                  string `json:"summary"`
			Description              string `json:"description"`
			CollaborationMode        string `json:"collaborationMode"`
			JoinMode                 string `json:"joinMode"`
			AcceptsSponsorship       bool   `json:"acceptsSponsorship"`
			AllowOthersManagement    bool   `json:"allowOthersManagement"`
			ConfidentialityRequested bool   `json:"confidentialityRequested"`
			ConfidentialityTerms     string `json:"confidentialityTerms"`
			EstimatedDurationDays    int    `json:"estimatedDurationDays"`
			LongTerm                 bool   `json:"longTerm"`
			Completed                bool   `json:"completed"`
			ExecutionStatus          string `json:"executionStatus"`
			PlannedStartDate         string `json:"plannedStartDate"`
			ScheduleNote             string `json:"scheduleNote"`
			ProposedDifficulty       string `json:"proposedDifficulty"`
			AdventureStyle           string `json:"adventureStyle"`
			ProjectNature            string `json:"projectNature"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid project.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Summary = strings.TrimSpace(input.Summary)
		input.Description = strings.TrimSpace(input.Description)
		input.ScheduleNote = strings.TrimSpace(input.ScheduleNote)
		input.ConfidentialityTerms = strings.TrimSpace(input.ConfidentialityTerms)
		if (input.ConfidentialityRequested && input.ConfidentialityTerms == "") || utf8.RuneCountInString(input.ConfidentialityTerms) > 10000 {
			return e.BadRequestError("Invalid confidentiality agreement.", nil)
		}
		if utf8.RuneCountInString(input.Description) > 10000 || utf8.RuneCountInString(input.ScheduleNote) > 2000 || input.EstimatedDurationDays < 0 || input.EstimatedDurationDays > 36500 {
			return e.BadRequestError("Invalid project details.", nil)
		}
		if input.ExecutionStatus == "" {
			if input.Completed {
				input.ExecutionStatus = "completed"
			} else {
				input.ExecutionStatus = "ongoing"
			}
		}
		if input.ExecutionStatus != "not_started" && input.ExecutionStatus != "ongoing" && input.ExecutionStatus != "completed" {
			return e.BadRequestError("Invalid project status.", nil)
		}
		if input.LongTerm && input.EstimatedDurationDays != 0 {
			return e.BadRequestError("Long-term projects cannot have an estimated duration.", nil)
		}
		if input.PlannedStartDate != "" {
			if _, err := time.Parse("2006-01-02", input.PlannedStartDate); err != nil {
				return e.BadRequestError("Invalid start date.", nil)
			}
		}
		if input.CollaborationMode == "" {
			input.CollaborationMode = "collaborative"
		}
		if input.JoinMode == "" {
			input.JoinMode = "open"
		}
		if (input.CollaborationMode != "independent" && input.CollaborationMode != "collaborative") || (input.JoinMode != "open" && input.JoinMode != "approval") {
			return e.BadRequestError("Invalid collaboration settings.", nil)
		}
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || utf8.RuneCountInString(input.Summary) > 2000 {
			return e.BadRequestError("Invalid project.", nil)
		}
		if input.ProposedDifficulty != "" && !validProjectDifficulty(input.ProposedDifficulty) {
			return e.BadRequestError("Invalid difficulty.", nil)
		}
		if input.ProjectNature != "" && input.ProjectNature != "commercial" && input.ProjectNature != "learning" && input.ProjectNature != "showcase" && input.ProjectNature != "research" {
			return e.BadRequestError("Invalid project purpose.", nil)
		}
		if input.AdventureStyle == "" {
			input.AdventureStyle = "project"
		}
		if input.AdventureStyle != "project" && input.AdventureStyle != "dungeon" && input.AdventureStyle != "boss" {
			return e.BadRequestError("Invalid adventure style.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("projects")
		if err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		project := core.NewRecord(collection)
		project.Set("creator_user_id", creator.Id)
		project.Set("owner_user_id", creator.Id)
		project.Set("title", input.Title)
		project.Set("summary", input.Summary)
		project.Set("review_status", "approved")
		project.Set("execution_status", input.ExecutionStatus)
		project.Set("is_long_term", input.LongTerm)
		project.Set("collaboration_mode", input.CollaborationMode)
		project.Set("join_mode", input.JoinMode)
		project.Set("accepts_sponsorship", input.AcceptsSponsorship)
		project.Set("allow_others_management", input.AllowOthersManagement)
		project.Set("confidentiality_requested", input.ConfidentialityRequested)
		if input.ConfidentialityRequested {
			project.Set("confidentiality_terms", input.ConfidentialityTerms)
		}
		project.Set("description", map[string]string{"text": input.Description})
		project.Set("schedule_note", input.ScheduleNote)
		if input.PlannedStartDate != "" {
			project.Set("planned_start_date", input.PlannedStartDate)
		}
		if input.EstimatedDurationDays > 0 {
			project.Set("estimated_duration_days", input.EstimatedDurationDays)
		}
		project.Set("adventure_style", input.AdventureStyle)
		project.Set("project_nature", input.ProjectNature)
		project.Set("version", 1)
		project.Set("published_at", time.Now())
		if input.ProposedDifficulty != "" {
			project.Set("owner_proposed_difficulty", input.ProposedDifficulty)
		}
		if err := e.App.Save(project); err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "id": project.Id})
	})
	e.Router.GET("/api/hpahub/me/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		initiated := []myProject{}
		participated := []myProject{}
		followed := []myProject{}
		seen := map[string]bool{}
		converted := func(project *core.Record) myProject {
			ownerName := ""
			ownerAvatarURL := ""
			ownerRedacted := false
			if owner, err := e.App.FindRecordById("users", project.GetString("owner_user_id")); err == nil {
				ownerName = publicPersonName(owner)
				ownerRedacted = profileSectionHidden(owner, "projects") || profileSectionHidden(owner, "project:"+project.Id)
				if ownerRedacted {
					ownerName = maskedText(ownerName)
				}
				if !ownerRedacted && owner.GetString("avatar") != "" {
					ownerAvatarURL = contentURL("members", owner.Id, "avatar")
				}
			}
			return myProject{
				OwnerRedacted:  ownerRedacted,
				ID:             project.Id,
				Title:          project.GetString("title"),
				Summary:        project.GetString("summary"),
				Status:         project.GetString("execution_status"),
				ReviewStatus:   project.GetString("review_status"),
				Difficulty:     project.GetString("difficulty"),
				OwnerName:      ownerName,
				OwnerAvatarURL: ownerAvatarURL,
				Updated:        project.GetString("updated"),
			}
		}
		owned, err := e.App.FindRecordsByFilter("projects", `owner_user_id = {:user} && deleted_at = ""`, "-updated", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, project := range owned {
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			initiated = append(initiated, converted(project))
		}
		memberships, err := e.App.FindRecordsByFilter("project_members", `user_id = {:user} && status = "active"`, "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, membership := range memberships {
			project, err := e.App.FindRecordById("projects", membership.GetString("project_id"))
			if err != nil || project.GetString("owner_user_id") == member.Id || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
				continue
			}
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			participated = append(participated, converted(project))
		}
		follows, err := e.App.FindRecordsByFilter("project_follows", `user_id = {:user} && is_active = true`, "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, follow := range follows {
			project, err := e.App.FindRecordById("projects", follow.GetString("project_id"))
			if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
				continue
			}
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			followed = append(followed, converted(project))
		}
		return e.JSON(200, map[string]any{
			"initiated":    initiated,
			"participated": participated,
			"followed":     followed,
		})
	})

	e.Router.POST("/api/hpahub/projects/{id}/follow", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You cannot follow your own project.", nil)
		}
		follow, err := e.App.FindFirstRecordByFilter("project_follows", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id})
		if errors.Is(err, sql.ErrNoRows) {
			collection, err := e.App.FindCollectionByNameOrId("project_follows")
			if err != nil {
				return e.InternalServerError("Unable to follow project.", err)
			}
			follow = core.NewRecord(collection)
			follow.Set("project_id", project.Id)
			follow.Set("user_id", viewer.Id)
			follow.Set("is_active", true)
			follow.Set("receive_updates", true)
			follow.Set("followed_at", time.Now())
		} else if err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		} else {
			follow.Set("is_active", !follow.GetBool("is_active"))
			if follow.GetBool("is_active") {
				follow.Set("followed_at", time.Now())
			}
		}
		if err := e.App.Save(follow); err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		}
		return e.JSON(200, map[string]any{"following": follow.GetBool("is_active")})
	})
}
