package main

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type memberTitle struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsOfficial    bool   `json:"isOfficial"`
	IssuedByName  string `json:"issuedByName,omitempty"`
	SourceProject string `json:"sourceProjectId,omitempty"`
	Created       string `json:"created"`
}

func memberTitlesWithIssuerNames(app core.App, rows []*core.Record) ([]memberTitle, error) {
	issuerIDs := map[string]bool{}
	for _, row := range rows {
		if id := row.GetString("issued_by"); id != "" {
			issuerIDs[id] = true
		}
	}
	issuers := map[string]string{}
	if len(issuerIDs) > 0 {
		filters := make([]string, 0, len(issuerIDs))
		params := dbx.Params{}
		i := 0
		for id := range issuerIDs {
			key := "issuer" + strconv.Itoa(i)
			i++
			filters = append(filters, "id = {:"+key+"}")
			params[key] = id
		}
		users, err := app.FindRecordsByFilter("users", strings.Join(filters, " || "), "", 0, 0, params)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			issuers[user.Id] = publicPersonName(user)
		}
	}
	result := make([]memberTitle, 0, len(rows))
	for _, row := range rows {
		result = append(result, memberTitle{
			ID: row.Id, Name: row.GetString("name"), IsOfficial: row.GetBool("is_official"),
			IssuedByName: issuers[row.GetString("issued_by")], SourceProject: row.GetString("source_project_id"), Created: row.GetString("created"),
		})
	}
	return result, nil
}

func listMemberTitles(app core.App, userID string) ([]memberTitle, error) {
	rows, err := app.FindRecordsByFilter("user_titles", "user_id = {:user} && revoked_at = \"\"", "created,id", 0, 0, dbx.Params{"user": userID})
	if err != nil {
		return nil, err
	}
	return memberTitlesWithIssuerNames(app, rows)
}

func listTitlesForMembers(app core.App, members []*core.Record) (map[string][]memberTitle, error) {
	result := make(map[string][]memberTitle, len(members))
	if len(members) == 0 {
		return result, nil
	}
	filters := make([]string, 0, len(members))
	params := dbx.Params{}
	for i, member := range members {
		key := "member" + strconv.Itoa(i)
		filters = append(filters, "user_id = {:"+key+"}")
		params[key] = member.Id
		result[member.Id] = []memberTitle{}
	}
	rows, err := app.FindRecordsByFilter("user_titles", "("+strings.Join(filters, " || ")+") && revoked_at = \"\"", "user_id,created,id", 0, 0, params)
	if err != nil {
		return nil, err
	}
	titles, err := memberTitlesWithIssuerNames(app, rows)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		id := row.GetString("user_id")
		result[id] = append(result[id], titles[i])
	}
	return result, nil
}

func bindMemberTitleRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/manage/members/{id}/titles", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		var input struct {
			Name string `json:"name"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid title.", err)
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || utf8.RuneCountInString(input.Name) > 10 {
			return e.BadRequestError("Title must be between 1 and 10 characters.", nil)
		}
		issuer, linked, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load issuer.", err)
		}
		var saved *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			if _, err := tx.FindRecordById("users", e.Request.PathValue("id")); err != nil {
				return e.NotFoundError("Member not found.", nil)
			}
			row, err := tx.FindFirstRecordByFilter("user_titles", "user_id = {:user} && name = {:name} && source_project_id = \"\" && revoked_at = \"\"", dbx.Params{"user": e.Request.PathValue("id"), "name": input.Name})
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				collection, findErr := tx.FindCollectionByNameOrId("user_titles")
				if findErr != nil {
					return findErr
				}
				row = core.NewRecord(collection)
				row.Set("user_id", e.Request.PathValue("id"))
				row.Set("name", input.Name)
			}
			row.Set("is_official", true)
			if linked {
				row.Set("issued_by", issuer.Id)
			}
			if err := tx.Save(row); err != nil {
				return err
			}
			saved = row
			return nil
		})
		if err != nil {
			return e.BadRequestError("Unable to publish official title.", err)
		}
		return e.JSON(200, memberTitle{ID: saved.Id, Name: saved.GetString("name"), IsOfficial: saved.GetBool("is_official"), Created: saved.GetString("created")})
	})

	e.Router.PATCH("/api/hpahub/manage/members/{id}/titles/{titleId}/revoke", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		if err := e.App.RunInTransaction(func(tx core.App) error {
			row, err := tx.FindRecordById("user_titles", e.Request.PathValue("titleId"))
			if err != nil || row.GetString("user_id") != e.Request.PathValue("id") {
				return e.NotFoundError("Title not found.", nil)
			}
			if row.GetString("revoked_at") == "" {
				row.Set("revoked_at", time.Now())
				return tx.Save(row)
			}
			return nil
		}); err != nil {
			return err
		}
		return e.JSON(200, map[string]bool{"ok": true})
	})

	e.Router.POST("/api/hpahub/projects/{id}/titles", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, linked, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !linked {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			UserID string `json:"userId"`
		}
		if err := e.BindBody(&input); err != nil || input.UserID == "" {
			return e.BadRequestError("Invalid member.", nil)
		}
		var saved *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := tx.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil || project.GetString("deleted_at") != "" {
				return e.NotFoundError("Not found.", nil)
			}
			if project.GetString("owner_user_id") != viewer.Id {
				return e.ForbiddenError("Only the project initiator can grant this title.", nil)
			}
			if project.GetString("review_status") != "approved" || project.GetString("execution_status") != "completed" {
				return e.BadRequestError("Project must be completed before granting its title.", nil)
			}
			hasOffer := false
			for _, offer := range project.GetStringSlice("provider_offers") {
				if offer == "title" {
					hasOffer = true
					break
				}
			}
			name := strings.TrimSpace(projectSelectionDetails(project, "provider_offer_details")["title"])
			if !hasOffer || name == "" || utf8.RuneCountInString(name) > 10 {
				return e.BadRequestError("Project title reward is not configured.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": input.UserID}); err != nil {
				return e.BadRequestError("Active project member required.", nil)
			}
			existing, err := tx.FindFirstRecordByFilter("user_titles", `user_id = {:user} && name = {:name} && source_project_id = {:project} && revoked_at = ""`, dbx.Params{"user": input.UserID, "name": name, "project": project.Id})
			if err == nil {
				saved = existing
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := tx.FindCollectionByNameOrId("user_titles")
			if err != nil {
				return err
			}
			saved = core.NewRecord(collection)
			saved.Set("user_id", input.UserID)
			saved.Set("name", name)
			saved.Set("is_official", false)
			saved.Set("issued_by", viewer.Id)
			saved.Set("source_project_id", project.Id)
			return tx.Save(saved)
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true, "titleId": saved.Id})
	})
}
