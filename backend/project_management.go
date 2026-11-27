package main

import (
	"database/sql"
	"errors"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"time"
)

func saveConfidentialityAgreement(app core.App, project, member *core.Record, accepted string) error {
	if !project.GetBool("confidentiality_requested") {
		return nil
	}
	terms := project.GetString("confidentiality_terms")
	if terms == "" || accepted != terms {
		return errors.New("agreement not accepted")
	}
	collection, err := app.FindCollectionByNameOrId("project_agreements")
	if err != nil {
		return err
	}
	agreement := core.NewRecord(collection)
	agreement.Set("project_id", project.Id)
	agreement.Set("participant_user_id", member.Id)
	agreement.Set("proposer_user_id", project.GetString("owner_user_id"))
	agreement.Set("terms_snapshot", map[string]string{"type": "confidentiality", "text": terms})
	agreement.Set("status", "confirmed")
	agreement.Set("participant_confirmed_at", time.Now())
	return app.Save(agreement)
}

func canManageProject(app core.App, project *core.Record, userID string) bool {
	if project.GetString("owner_user_id") == userID {
		return true
	}
	if !project.GetBool("allow_others_management") {
		return false
	}
	if _, err := app.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": userID}); err != nil {
		return false
	}
	_, err := app.FindFirstRecordByFilter("project_permission_grants", `project_id = {:project} && user_id = {:user} && permission_code = "manage" && revoked_at = ""`, dbx.Params{"project": project.Id, "user": userID})
	return err == nil
}

func bindProjectManagementRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/projects/{id}/management", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !ok {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if !canManageProject(e.App, project, viewer.Id) {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		members, err := e.App.FindRecordsByFilter("project_members", `project_id = {:project} && status = "active"`, "joined_at", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load members.", err)
		}
		memberItems := []map[string]any{}
		for _, membership := range members {
			user, err := e.App.FindRecordById("users", membership.GetString("user_id"))
			if err != nil {
				continue
			}
			name := publicPersonName(user)
			if profileSectionHidden(user, "projects") || profileSectionHidden(user, "project:"+project.Id) {
				name = maskedText(name)
			}
			memberItems = append(memberItems, map[string]any{"id": user.Id, "name": name, "manager": canManageProject(e.App, project, user.Id)})
		}
		requests, err := e.App.FindRecordsByFilter("project_join_requests", `project_id = {:project} && status = "pending"`, "created", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load applications.", err)
		}
		requestItems := []map[string]any{}
		for _, request := range requests {
			user, err := e.App.FindRecordById("users", request.GetString("user_id"))
			if err != nil {
				continue
			}
			name := publicPersonName(user)
			if profileSectionHidden(user, "projects") || profileSectionHidden(user, "project:"+project.Id) {
				name = maskedText(name)
			}
			requestItems = append(requestItems, map[string]any{"id": request.Id, "name": name, "message": request.GetString("message"), "source": request.GetString("source")})
		}
		return e.JSON(200, map[string]any{"members": memberItems, "requests": requestItems, "canAssign": project.GetBool("allow_others_management"), "viewerId": viewer.Id})
	})
	e.Router.PATCH("/api/hpahub/projects/{id}/managers/{user}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !ok {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			Manager bool `json:"manager"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid permission.", err)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := tx.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil || project.GetString("deleted_at") != "" {
				return e.NotFoundError("Not found.", nil)
			}
			if !project.GetBool("allow_others_management") || !canManageProject(tx, project, viewer.Id) {
				return e.ForbiddenError("Insufficient permissions.", nil)
			}
			target := e.Request.PathValue("user")
			if target == project.GetString("owner_user_id") || target == viewer.Id {
				return e.BadRequestError("Cannot change this manager.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": target}); err != nil {
				return e.BadRequestError("Active project member required.", nil)
			}
			grant, err := tx.FindFirstRecordByFilter("project_permission_grants", `project_id = {:project} && user_id = {:user} && permission_code = "manage"`, dbx.Params{"project": project.Id, "user": target})
			if errors.Is(err, sql.ErrNoRows) {
				if !input.Manager {
					return nil
				}
				collection, err := tx.FindCollectionByNameOrId("project_permission_grants")
				if err != nil {
					return err
				}
				grant = core.NewRecord(collection)
				grant.Set("project_id", project.Id)
				grant.Set("user_id", target)
				grant.Set("permission_code", "manage")
			} else if err != nil {
				return err
			}
			grant.Set("granted_by", viewer.Id)
			grant.Set("granted_at", time.Now())
			if input.Manager {
				grant.Set("revoked_at", "")
			} else {
				grant.Set("revoked_at", time.Now())
			}
			return tx.Save(grant)
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.PATCH("/api/hpahub/projects/{id}/applications/{request}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !ok {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			Status string `json:"status"`
		}
		if err := e.BindBody(&input); err != nil || (input.Status != "accepted" && input.Status != "rejected") {
			return e.BadRequestError("Invalid application status.", nil)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := tx.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil || project.GetString("deleted_at") != "" {
				return e.NotFoundError("Not found.", nil)
			}
			if !canManageProject(tx, project, viewer.Id) {
				return e.ForbiddenError("Insufficient permissions.", nil)
			}
			request, err := tx.FindRecordById("project_join_requests", e.Request.PathValue("request"))
			if err != nil || request.GetString("project_id") != project.Id {
				return e.NotFoundError("Not found.", nil)
			}
			if request.GetString("status") != "pending" {
				return e.BadRequestError("Application already handled.", nil)
			}
			if input.Status == "accepted" {
				userID := request.GetString("user_id")
				member, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user}`, dbx.Params{"project": project.Id, "user": userID})
				if errors.Is(err, sql.ErrNoRows) {
					collection, err := tx.FindCollectionByNameOrId("project_members")
					if err != nil {
						return err
					}
					member = core.NewRecord(collection)
					member.Set("project_id", project.Id)
					member.Set("user_id", userID)
				} else if err != nil {
					return err
				}
				member.Set("status", "active")
				member.Set("joined_at", time.Now())
				if err := tx.Save(member); err != nil {
					return err
				}
			}
			request.Set("status", input.Status)
			request.Set("handler_user_id", viewer.Id)
			request.Set("handled_at", time.Now())
			return tx.Save(request)
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
}
