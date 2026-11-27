package main

import (
	"database/sql"
	"errors"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"strings"
	"time"
)

func joinRequestDuties(request *core.Record) string {
	parts := []string{}
	if value := strings.TrimSpace(request.GetString("apply_profession")); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(request.GetString("apply_ability")); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, " · ")
}

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
	bindMemberDepartureRoutes(e)
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
		departures, err := projectDepartureItems(e.App, project.Id)
		if err != nil {
			return e.InternalServerError("Unable to load member departure requests.", err)
		}
		departuresByMember := memberDepartureMap(departures)
		memberUsers := map[string]*core.Record{}
		memberRecords := make([]*core.Record, 0, len(members))
		for _, membership := range members {
			if user, findErr := e.App.FindRecordById("users", membership.GetString("user_id")); findErr == nil {
				memberUsers[user.Id] = user
				memberRecords = append(memberRecords, user)
			}
		}
		titlesByMember, err := listTitlesForMembers(e.App, memberRecords)
		if err != nil {
			return e.InternalServerError("Unable to load member titles.", err)
		}
		memberItems := []map[string]any{}
		for _, membership := range members {
			user := memberUsers[membership.GetString("user_id")]
			if user == nil {
				continue
			}
			name := publicPersonName(user)
			if profileSectionHidden(user, "projects") || profileSectionHiddenForViewer(e.App, user, "project:"+project.Id, e.Auth) {
				name = maskedText(name)
			}
			memberData := map[string]any{"id": user.Id, "name": name, "manager": canManageProject(e.App, project, user.Id)}
			for _, title := range titlesByMember[user.Id] {
				if title.SourceProject == project.Id && title.Name == projectSelectionDetails(project, "provider_offer_details")["title"] {
					memberData["titleAwarded"] = true
					break
				}
			}
			if departure, ok := departuresByMember[user.Id]; ok {
				memberData["departureRequest"] = departure
			}
			memberItems = append(memberItems, memberData)
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
			if profileSectionHidden(user, "projects") || profileSectionHiddenForViewer(e.App, user, "project:"+project.Id, e.Auth) {
				name = maskedText(name)
			}
			requestItems = append(requestItems, map[string]any{"id": request.Id, "name": name, "message": request.GetString("message"), "source": request.GetString("source"), "profession": request.GetString("apply_profession"), "ability": request.GetString("apply_ability"), "time": request.GetString("apply_time")})
		}
		titleOffer := ""
		for _, offer := range project.GetStringSlice("provider_offers") {
			if offer == "title" {
				titleOffer = projectSelectionDetails(project, "provider_offer_details")["title"]
				break
			}
		}
		completionRows, err := e.App.FindRecordsByFilter("project_completion_requests", `project_id = {:project} && status = "pending"`, "created", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load completion requests.", err)
		}
		completionItems := []map[string]any{}
		for _, request := range completionRows {
			user, err := e.App.FindRecordById("users", request.GetString("user_id"))
			if err != nil {
				continue
			}
			name := publicPersonName(user)
			if profileSectionHidden(user, "projects") || profileSectionHiddenForViewer(e.App, user, "project:"+project.Id, e.Auth) {
				name = maskedText(name)
			}
			completionItems = append(completionItems, map[string]any{"id": request.Id, "userId": user.Id, "name": name, "number": request.GetInt("completion_number"), "message": request.GetString("message")})
		}
		return e.JSON(200, map[string]any{"members": memberItems, "requests": requestItems, "completionRequests": completionItems, "departureRequests": departures, "canAssign": project.GetBool("allow_others_management"), "isOwner": project.GetString("owner_user_id") == viewer.Id, "viewerId": viewer.Id, "titleAwardName": titleOffer, "titleAwardAvailable": titleOffer != "" && project.GetString("execution_status") == "completed" && project.GetString("review_status") == "approved"})
	})
	e.Router.POST("/api/hpahub/projects/{id}/completions", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !ok {
			return e.ForbiddenError("Active member profile required.", nil)
		}
		var input struct {
			Message string `json:"message"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid completion request.", err)
		}
		input.Message = strings.TrimSpace(input.Message)
		if input.Message == "" || len([]rune(input.Message)) > 5000 {
			return e.BadRequestError("Describe the completed task in 1 to 5000 characters.", nil)
		}
		var number int
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := tx.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil || project.GetString("deleted_at") != "" || project.GetString("review_status") != "approved" {
				return e.NotFoundError("Not found.", nil)
			}
			if !project.GetBool("is_repeatable") || project.GetString("execution_status") == "completed" || project.GetString("execution_status") == "terminated" {
				return e.BadRequestError("This project does not accept completion requests.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err != nil {
				return e.ForbiddenError("Active project membership required.", nil)
			}
			approved, err := tx.FindRecordsByFilter("project_completion_requests", `project_id = {:project} && user_id = {:user} && status = "approved"`, "", 0, 0, dbx.Params{"project": project.Id, "user": viewer.Id})
			if err != nil {
				return err
			}
			limit := max(project.GetInt("repeat_limit"), 1)
			if len(approved) >= limit {
				return e.BadRequestError("You have reached this project's completion limit.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_completion_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
				return e.BadRequestError("You already have a pending completion request.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := tx.FindCollectionByNameOrId("project_completion_requests")
			if err != nil {
				return err
			}
			request := core.NewRecord(collection)
			request.Set("project_id", project.Id)
			request.Set("user_id", viewer.Id)
			request.Set("completion_number", len(approved)+1)
			request.Set("message", input.Message)
			request.Set("status", "pending")
			number = len(approved) + 1
			return tx.Save(request)
		})
		if err != nil {
			return e.BadRequestError("Unable to submit completion request.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "number": number})
	})
	e.Router.PATCH("/api/hpahub/projects/{id}/completion-requests/{request}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !ok {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			Decision string `json:"decision"`
		}
		if err := e.BindBody(&input); err != nil || (input.Decision != "approve" && input.Decision != "reject") {
			return e.BadRequestError("Invalid completion decision.", nil)
		}
		var status string
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := tx.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil || !canManageProject(tx, project, viewer.Id) {
				return e.ForbiddenError("Insufficient permissions.", nil)
			}
			request, err := tx.FindRecordById("project_completion_requests", e.Request.PathValue("request"))
			if err != nil || request.GetString("project_id") != project.Id || request.GetString("status") != "pending" {
				return e.NotFoundError("Pending request not found.", nil)
			}
			status = "rejected"
			if input.Decision == "approve" {
				if !project.GetBool("is_repeatable") || project.GetString("execution_status") == "completed" || project.GetString("execution_status") == "terminated" {
					return e.BadRequestError("This project no longer accepts completions.", nil)
				}
				approved, err := tx.FindRecordsByFilter("project_completion_requests", `project_id = {:project} && user_id = {:user} && status = "approved"`, "", 0, 0, dbx.Params{"project": project.Id, "user": request.GetString("user_id")})
				if err != nil {
					return err
				}
				if len(approved) >= max(project.GetInt("repeat_limit"), 1) {
					return e.BadRequestError("The participant has reached the completion limit.", nil)
				}
				if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": request.GetString("user_id")}); err != nil {
					return e.BadRequestError("The participant is no longer an active member.", nil)
				}
				status = "approved"
			}
			request.Set("status", status)
			request.Set("reviewer_user_id", viewer.Id)
			request.Set("reviewed_at", time.Now().UTC())
			return tx.Save(request)
		})
		if err != nil {
			return e.BadRequestError("Unable to review completion request.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "status": status})
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
				if duties := joinRequestDuties(request); duties != "" {
					member.Set("duties", duties)
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
