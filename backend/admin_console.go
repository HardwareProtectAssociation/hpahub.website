package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type managedMember struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Email              string   `json:"email"`
	Role               string   `json:"role"`
	AccountStatus      string   `json:"accountStatus"`
	RegistrationSource string   `json:"registrationSource"`
	ProfilePublic      bool     `json:"profilePublic"`
	Created            string   `json:"created"`
	ActivityAreas      []string `json:"activityAreas"`
	Professions        []string `json:"professions"`
}

type managedApplication struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	ProjectTitle string `json:"projectTitle"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	UserEmail    string `json:"userEmail"`
	Message      string `json:"message"`
	Created      string `json:"created"`
}

func bindAdminConsoleRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/manage/members", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		rows, err := e.App.FindRecordsByFilter("users", `role = "member" || role = "admin"`, "created", 500, 0)
		if err != nil {
			return e.InternalServerError("Unable to load members.", err)
		}
		identities, err := e.App.FindRecordsByFilter("collaboration_identities", "", "name", 0, 0)
		if err != nil {
			return e.InternalServerError("Unable to load members.", err)
		}
		identityNames := map[string]string{}
		for _, identity := range identities {
			identityNames[identity.Id] = identity.GetString("name")
		}
		featured, err := e.App.FindRecordsByFilter("user_identities", "featured = true", "", 0, 0)
		if err != nil {
			return e.InternalServerError("Unable to load members.", err)
		}
		professions := map[string][]string{}
		for _, row := range featured {
			professions[row.GetString("user_id")] = append(professions[row.GetString("user_id")], identityNames[row.GetString("identity_id")])
		}
		items := make([]managedMember, 0, len(rows))
		for _, member := range rows {
			professionNames := professions[member.Id]
			if professionNames == nil {
				professionNames = []string{}
			}
			areas := parseCenterStrings(member, "activity_areas")
			if areas == nil {
				areas = []string{}
			}
			items = append(items, managedMember{
				ID:                 member.Id,
				Name:               publicPersonName(member),
				Email:              member.Email(),
				Role:               siteRole(member),
				AccountStatus:      member.GetString("account_status"),
				RegistrationSource: member.GetString("registration_source"),
				ProfilePublic:      member.GetBool("profilePublic"),
				Created:            member.GetString("created"),
				ActivityAreas:      areas,
				Professions:        professionNames,
			})
		}
		return e.JSON(200, map[string]any{"items": items})
	})

	e.Router.GET("/api/hpahub/manage/applications", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		rows, err := e.App.FindRecordsByFilter("project_join_requests", `status = "pending"`, "created", 200, 0)
		if err != nil {
			return e.InternalServerError("Unable to load applications.", err)
		}
		items := make([]managedApplication, 0, len(rows))
		for _, request := range rows {
			project, _ := e.App.FindRecordById("projects", request.GetString("project_id"))
			user, _ := e.App.FindRecordById("users", request.GetString("user_id"))
			item := managedApplication{
				ID:        request.Id,
				ProjectID: request.GetString("project_id"),
				Message:   request.GetString("message"),
				Created:   request.GetString("created"),
			}
			if project != nil {
				item.ProjectTitle = project.GetString("title")
			}
			if user != nil {
				item.UserID = user.Id
				item.UserName = publicPersonName(user)
				item.UserEmail = user.Email()
			}
			items = append(items, item)
		}
		return e.JSON(200, map[string]any{"items": items})
	})

	e.Router.PATCH("/api/hpahub/manage/applications/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		var input struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := e.BindBody(&input); err != nil || (input.Status != "accepted" && input.Status != "rejected") {
			return e.BadRequestError("Invalid application status.", nil)
		}
		request, err := e.App.FindRecordById("project_join_requests", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		if request.GetString("status") != "pending" {
			return e.BadRequestError("Application already handled.", nil)
		}
		handlerID := ""
		if owner, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member {
			handlerID = owner.Id
		}
		request.Set("status", input.Status)
		request.Set("handler_note", input.Note)
		request.Set("handled_at", time.Now())
		if handlerID != "" {
			request.Set("handler_user_id", handlerID)
		}
		if err := e.App.Save(request); err != nil {
			return e.InternalServerError("Unable to save application.", err)
		}
		if input.Status == "accepted" {
			projectID := request.GetString("project_id")
			userID := request.GetString("user_id")
			membership, err := e.App.FindFirstRecordByFilter("project_members", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": projectID, "user": userID})
			if errors.Is(err, sql.ErrNoRows) {
				collection, err := e.App.FindCollectionByNameOrId("project_members")
				if err != nil {
					return e.InternalServerError("Unable to add project member.", err)
				}
				membership = core.NewRecord(collection)
				membership.Set("project_id", projectID)
				membership.Set("user_id", userID)
				membership.Set("status", "active")
				membership.Set("joined_at", time.Now())
			} else if err != nil {
				return e.InternalServerError("Unable to add project member.", err)
			} else {
				membership.Set("status", "active")
				membership.Set("joined_at", time.Now())
			}
			if err := e.App.Save(membership); err != nil {
				return e.InternalServerError("Unable to add project member.", err)
			}
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
}

func requireConsoleAdmin(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.UnauthorizedError("Authentication required.", nil)
	}
	role := siteRole(e.Auth)
	if role != "admin" && role != "octopus" {
		return e.ForbiddenError("Insufficient permissions.", nil)
	}
	return nil
}
