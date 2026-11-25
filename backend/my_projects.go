package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type myProject struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Status       string `json:"status"`
	ReviewStatus string `json:"reviewStatus"`
	Difficulty   string `json:"difficulty"`
	OwnerName    string `json:"ownerName"`
	Updated      string `json:"updated"`
}

func bindMyProjectRoutes(e *core.ServeEvent) {
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
			if owner, err := e.App.FindRecordById("users", project.GetString("owner_user_id")); err == nil {
				ownerName = publicPersonName(owner)
			}
			return myProject{
				ID:           project.Id,
				Title:        project.GetString("title"),
				Summary:      project.GetString("summary"),
				Status:       project.GetString("execution_status"),
				ReviewStatus: project.GetString("review_status"),
				Difficulty:   project.GetString("difficulty"),
				OwnerName:    ownerName,
				Updated:      project.GetString("updated"),
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
