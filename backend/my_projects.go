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
			Title              string `json:"title"`
			Summary            string `json:"summary"`
			ProposedDifficulty string `json:"proposedDifficulty"`
			AdventureStyle     string `json:"adventureStyle"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid project.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Summary = strings.TrimSpace(input.Summary)
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || utf8.RuneCountInString(input.Summary) > 2000 {
			return e.BadRequestError("Invalid project.", nil)
		}
		if input.ProposedDifficulty != "" && !validProjectDifficulty(input.ProposedDifficulty) {
			return e.BadRequestError("Invalid difficulty.", nil)
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
		project.Set("execution_status", "ongoing")
		project.Set("collaboration_mode", "collaborative")
		project.Set("join_mode", "open")
		project.Set("adventure_style", input.AdventureStyle)
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
			if owner, err := e.App.FindRecordById("users", project.GetString("owner_user_id")); err == nil {
				ownerName = publicPersonName(owner)
				if owner.GetString("avatar") != "" {
					ownerAvatarURL = contentURL("members", owner.Id, "avatar")
				}
			}
			return myProject{
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
