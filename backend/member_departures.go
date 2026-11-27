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

const departureWindow = 48 * time.Hour

type memberDeparture struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	MemberID    string `json:"memberId"`
	MemberName  string `json:"memberName"`
	RequesterID string `json:"requesterId"`
	Requester   string `json:"requesterName"`
	Type        string `json:"type"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt"`
	Created     string `json:"created"`
}

func departureData(app core.App, row *core.Record) memberDeparture {
	result := memberDeparture{ID: row.Id, ProjectID: row.GetString("project_id"), MemberID: row.GetString("member_user_id"), RequesterID: row.GetString("requester_user_id"), Type: row.GetString("request_type"), Reason: row.GetString("reason"), Status: row.GetString("status"), ExpiresAt: row.GetString("expires_at"), Created: row.GetString("created")}
	if project, err := app.FindRecordById("projects", result.ProjectID); err == nil {
		result.ProjectName = project.GetString("title")
	}
	if member, err := app.FindRecordById("users", result.MemberID); err == nil {
		result.MemberName = publicPersonName(member)
	}
	if requester, err := app.FindRecordById("users", result.RequesterID); err == nil {
		result.Requester = publicPersonName(requester)
	}
	return result
}

func openDepartureForMember(app core.App, projectID, memberID string) (*core.Record, error) {
	return app.FindFirstRecordByFilter("project_member_departure_requests", `project_id = {:project} && member_user_id = {:member} && (status = "pending_owner" || status = "pending_member" || status = "pending_admin")`, dbx.Params{"project": projectID, "member": memberID})
}

func promoteExpiredDepartures(app core.App) error {
	return promoteExpiredProjectDepartures(app, "")
}

func promoteExpiredProjectDepartures(app core.App, projectID string) error {
	now := time.Now()
	filter := `(status = "pending_owner" || status = "pending_member") && expires_at <= {:now}`
	params := dbx.Params{"now": now}
	if projectID != "" {
		filter += " && project_id = {:project}"
		params["project"] = projectID
	}
	rows, err := app.FindRecordsByFilter("project_member_departure_requests", filter, "expires_at,id", 0, 0, params)
	if err != nil {
		return err
	}
	for _, candidate := range rows {
		if err := app.RunInTransaction(func(tx core.App) error {
			row, err := tx.FindRecordById("project_member_departure_requests", candidate.Id)
			if err != nil || (row.GetString("status") != "pending_owner" && row.GetString("status") != "pending_member") || row.GetDateTime("expires_at").Time().After(now) {
				return err
			}
			row.Set("status", "pending_admin")
			row.Set("escalated_at", now)
			if err := tx.Save(row); err != nil {
				return err
			}
			project, _ := tx.FindRecordById("projects", row.GetString("project_id"))
			memberID, requesterID := row.GetString("member_user_id"), row.GetString("requester_user_id")
			subject := "项目成员申请已转交管理员审核"
			content := "项目「" + project.GetString("title") + "」的成员退出/请离申请已超过 48 小时，现已转交管理员审核。"
			if err := sendSystemMessage(tx, memberID, subject, content, "project_departure", row.GetString("project_id")); err != nil {
				return err
			}
			if requesterID != memberID {
				if err := sendSystemMessage(tx, requesterID, subject, content, "project_departure", row.GetString("project_id")); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func bindMemberDepartureRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/projects/{id}/leave-requests", func(e *core.RequestEvent) error {
		noCache(e)
		member, err := departureViewer(e)
		if err != nil {
			return err
		}
		var input struct {
			Reason string `json:"reason"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid leave request.", err)
		}
		input.Reason = strings.TrimSpace(input.Reason)
		if input.Reason == "" || utf8.RuneCountInString(input.Reason) > 2000 {
			return e.BadRequestError("A reason of up to 2000 characters is required.", nil)
		}
		var result *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := departureProject(tx, e.Request.PathValue("id"))
			if err != nil {
				return e.NotFoundError("Not found.", nil)
			}
			if project.GetString("owner_user_id") == member.Id {
				return e.BadRequestError("The project owner cannot leave their own project.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:member} && status = "active"`, dbx.Params{"project": project.Id, "member": member.Id}); err != nil {
				return e.BadRequestError("You are not an active project member.", nil)
			}
			if _, err := openDepartureForMember(tx, project.Id, member.Id); err == nil {
				return e.BadRequestError("A member departure request is already open.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := tx.FindCollectionByNameOrId("project_member_departure_requests")
			if err != nil {
				return err
			}
			result = core.NewRecord(collection)
			result.Set("project_id", project.Id)
			result.Set("member_user_id", member.Id)
			result.Set("requester_user_id", member.Id)
			result.Set("request_type", "leave")
			result.Set("reason", input.Reason)
			result.Set("open_request_key", project.Id+"/"+member.Id)
			result.Set("status", "pending_owner")
			result.Set("expires_at", time.Now().Add(departureWindow))
			if err := tx.Save(result); err != nil {
				return err
			}
			return sendSystemMessage(tx, project.GetString("owner_user_id"), "项目成员申请退出", "有成员申请退出项目「"+project.GetString("title")+"」，请在 48 小时内处理。", "project_departure", project.Id)
		})
		if err != nil {
			return err
		}
		return e.JSON(201, departureData(e.App, result))
	})

	e.Router.POST("/api/hpahub/projects/{id}/members/{user}/removal-requests", func(e *core.RequestEvent) error {
		noCache(e)
		owner, err := departureViewer(e)
		if err != nil {
			return err
		}
		var input struct {
			Reason string `json:"reason"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid removal request.", err)
		}
		input.Reason = strings.TrimSpace(input.Reason)
		if input.Reason == "" || utf8.RuneCountInString(input.Reason) > 2000 {
			return e.BadRequestError("A reason of up to 2000 characters is required.", nil)
		}
		var result *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := departureProject(tx, e.Request.PathValue("id"))
			if err != nil {
				return e.NotFoundError("Not found.", nil)
			}
			if project.GetString("owner_user_id") != owner.Id {
				return e.ForbiddenError("Only the project initiator can request a member removal.", nil)
			}
			target := e.Request.PathValue("user")
			if target == owner.Id {
				return e.BadRequestError("The project initiator cannot be removed.", nil)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:member} && status = "active"`, dbx.Params{"project": project.Id, "member": target}); err != nil {
				return e.BadRequestError("An active project member is required.", nil)
			}
			if _, err := openDepartureForMember(tx, project.Id, target); err == nil {
				return e.BadRequestError("A member departure request is already open.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := tx.FindCollectionByNameOrId("project_member_departure_requests")
			if err != nil {
				return err
			}
			result = core.NewRecord(collection)
			result.Set("project_id", project.Id)
			result.Set("member_user_id", target)
			result.Set("requester_user_id", owner.Id)
			result.Set("request_type", "remove")
			result.Set("reason", input.Reason)
			result.Set("open_request_key", project.Id+"/"+target)
			result.Set("status", "pending_member")
			result.Set("expires_at", time.Now().Add(departureWindow))
			if err := tx.Save(result); err != nil {
				return err
			}
			return sendSystemMessage(tx, target, "项目发起人申请请离参与者", "项目「"+project.GetString("title")+"」的发起人申请请你离开项目，请在 48 小时内同意或拒绝。", "project_departure", project.Id)
		})
		if err != nil {
			return err
		}
		return e.JSON(201, departureData(e.App, result))
	})

	e.Router.PATCH("/api/hpahub/projects/{id}/departure-requests/{request}/owner-response", func(e *core.RequestEvent) error {
		noCache(e)
		viewer, err := departureViewer(e)
		if err != nil {
			return err
		}
		var input struct {
			Decision string `json:"decision"`
		}
		if err := e.BindBody(&input); err != nil || (input.Decision != "approve" && input.Decision != "reject") {
			return e.BadRequestError("Decision must be approve or reject.", nil)
		}
		var result *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := departureProject(tx, e.Request.PathValue("id"))
			if err != nil || project.GetString("owner_user_id") != viewer.Id {
				return e.ForbiddenError("Only the project initiator can review this request.", nil)
			}
			result, err = tx.FindRecordById("project_member_departure_requests", e.Request.PathValue("request"))
			if err != nil || result.GetString("project_id") != project.Id || result.GetString("request_type") != "leave" {
				return e.NotFoundError("Request not found.", nil)
			}
			if result.GetString("status") != "pending_owner" {
				return e.BadRequestError("This request is no longer awaiting the project initiator.", nil)
			}
			if result.GetDateTime("expires_at").Time().Before(time.Now()) {
				return escalateDeparture(tx, result, "发起人未在48小时内处理，申请已转交管理员审核。")
			}
			if input.Decision == "approve" {
				return resolveDeparture(tx, result, "approved", viewer.Id, "")
			}
			return escalateDeparture(tx, result, "发起人拒绝了退出申请。")
		})
		if err != nil {
			return err
		}
		return e.JSON(200, departureData(e.App, result))
	})

	e.Router.PATCH("/api/hpahub/projects/{id}/departure-requests/{request}/member-response", func(e *core.RequestEvent) error {
		noCache(e)
		viewer, err := departureViewer(e)
		if err != nil {
			return err
		}
		var input struct {
			Decision string `json:"decision"`
		}
		if err := e.BindBody(&input); err != nil || (input.Decision != "approve" && input.Decision != "reject") {
			return e.BadRequestError("Decision must be approve or reject.", nil)
		}
		var result *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			project, err := departureProject(tx, e.Request.PathValue("id"))
			if err != nil {
				return e.NotFoundError("Not found.", nil)
			}
			result, err = tx.FindRecordById("project_member_departure_requests", e.Request.PathValue("request"))
			if err != nil || result.GetString("project_id") != project.Id || result.GetString("member_user_id") != viewer.Id || result.GetString("request_type") != "remove" {
				return e.NotFoundError("Request not found.", nil)
			}
			if result.GetString("status") != "pending_member" {
				return e.BadRequestError("This request is no longer awaiting your response.", nil)
			}
			if result.GetDateTime("expires_at").Time().Before(time.Now()) {
				return escalateDeparture(tx, result, "参与者未在48小时内处理，申请已转交管理员审核。")
			}
			if input.Decision == "approve" {
				return resolveDeparture(tx, result, "approved", viewer.Id, "")
			}
			return escalateDeparture(tx, result, "参与者拒绝了请离申请。")
		})
		if err != nil {
			return err
		}
		return e.JSON(200, departureData(e.App, result))
	})

	e.Router.GET("/api/hpahub/manage/departure-requests", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		if err := promoteExpiredDepartures(e.App); err != nil {
			return e.InternalServerError("Unable to escalate expired requests.", err)
		}
		offset, err := adminOffset(e)
		if err != nil {
			return err
		}
		rows, err := e.App.FindRecordsByFilter("project_member_departure_requests", `status = "pending_admin"`, "escalated_at,id", 51, offset)
		if err != nil {
			return e.InternalServerError("Unable to load departure requests.", err)
		}
		hasMore := len(rows) > 50
		if hasMore {
			rows = rows[:50]
		}
		items := make([]memberDeparture, 0, len(rows))
		for _, row := range rows {
			items = append(items, departureData(e.App, row))
		}
		return e.JSON(200, map[string]any{"items": items, "hasMore": hasMore})
	})

	e.Router.PATCH("/api/hpahub/manage/departure-requests/{request}", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		var input struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if err := e.BindBody(&input); err != nil || (input.Decision != "approve" && input.Decision != "reject") || utf8.RuneCountInString(input.Note) > 2000 {
			return e.BadRequestError("Invalid administrator decision.", nil)
		}
		viewer, linked, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load reviewer.", err)
		}
		reviewerID := ""
		if linked {
			reviewerID = viewer.Id
		}
		var result *core.Record
		err = e.App.RunInTransaction(func(tx core.App) error {
			result, err = tx.FindRecordById("project_member_departure_requests", e.Request.PathValue("request"))
			if err != nil || result.GetString("status") != "pending_admin" {
				return e.NotFoundError("Pending administrator review not found.", nil)
			}
			status := "rejected"
			if input.Decision == "approve" {
				status = "approved"
			}
			return resolveDeparture(tx, result, status, reviewerID, strings.TrimSpace(input.Note))
		})
		if err != nil {
			return err
		}
		return e.JSON(200, departureData(e.App, result))
	})
}

func departureViewer(e *core.RequestEvent) (*core.Record, error) {
	if e.Auth == nil {
		return nil, e.UnauthorizedError("Authentication required.", nil)
	}
	viewer, member, err := personalCenterOwner(e.App, e.Auth)
	if err != nil {
		return nil, e.InternalServerError("Unable to load member profile.", err)
	}
	if !member {
		return nil, e.ForbiddenError("A member profile is required.", nil)
	}
	return viewer, nil
}

func departureProject(app core.App, id string) (*core.Record, error) {
	project, err := app.FindRecordById("projects", id)
	if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
		return nil, sql.ErrNoRows
	}
	return project, nil
}

func escalateDeparture(app core.App, row *core.Record, note string) error {
	now := time.Now()
	row.Set("status", "pending_admin")
	row.Set("escalated_at", now)
	row.Set("review_note", note)
	if err := app.Save(row); err != nil {
		return err
	}
	project, _ := app.FindRecordById("projects", row.GetString("project_id"))
	content := "项目「" + project.GetString("title") + "」的退出/请离申请已转交管理员审核。"
	if err := sendSystemMessage(app, row.GetString("member_user_id"), "项目申请已转交管理员审核", content, "project_departure", project.Id); err != nil {
		return err
	}
	if row.GetString("requester_user_id") != row.GetString("member_user_id") {
		return sendSystemMessage(app, row.GetString("requester_user_id"), "项目申请已转交管理员审核", content, "project_departure", project.Id)
	}
	return nil
}

func resolveDeparture(app core.App, row *core.Record, status, reviewerID, note string) error {
	if status != "approved" && status != "rejected" {
		return errors.New("invalid departure resolution")
	}
	now := time.Now()
	if status == "approved" {
		member, err := app.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:member} && status = "active"`, dbx.Params{"project": row.GetString("project_id"), "member": row.GetString("member_user_id")})
		if err != nil {
			return errors.New("project membership is no longer active")
		}
		if row.GetString("request_type") == "leave" {
			member.Set("status", "left")
		} else {
			member.Set("status", "removed")
		}
		member.Set("left_at", now)
		if err := app.Save(member); err != nil {
			return err
		}
	}
	row.Set("status", status)
	row.Set("reviewer_user_id", reviewerID)
	row.Set("reviewed_at", now)
	row.Set("resolved_at", now)
	row.Set("review_note", note)
	row.Set("open_request_key", "")
	if err := app.Save(row); err != nil {
		return err
	}
	project, _ := app.FindRecordById("projects", row.GetString("project_id"))
	message := "项目「" + project.GetString("title") + "」的退出/请离申请已由管理员处理。"
	if row.GetString("request_type") == "leave" {
		if status == "approved" {
			message = "项目发起人/管理员已同意你退出项目「" + project.GetString("title") + "」。"
		} else {
			message = "你的退出项目「" + project.GetString("title") + "」申请未获批准，你仍是项目成员。"
		}
	} else if status == "approved" {
		message = "项目请离申请已获同意，你已离开项目「" + project.GetString("title") + "」。"
	} else {
		message = "项目请离申请未获批准，你仍是项目成员。"
	}
	if err := sendSystemMessage(app, row.GetString("member_user_id"), "项目成员变更申请已处理", message, "project_departure", project.Id); err != nil {
		return err
	}
	if row.GetString("requester_user_id") != row.GetString("member_user_id") {
		return sendSystemMessage(app, row.GetString("requester_user_id"), "项目成员变更申请已处理", message, "project_departure", project.Id)
	}
	return nil
}

func projectDepartureItems(app core.App, projectID string) ([]memberDeparture, error) {
	if err := promoteExpiredProjectDepartures(app, projectID); err != nil {
		return nil, err
	}
	rows, err := app.FindRecordsByFilter("project_member_departure_requests", `project_id = {:project} && (status = "pending_owner" || status = "pending_member" || status = "pending_admin")`, "created,id", 0, 0, dbx.Params{"project": projectID})
	if err != nil {
		return nil, err
	}
	items := make([]memberDeparture, 0, len(rows))
	for _, row := range rows {
		items = append(items, departureData(app, row))
	}
	return items, nil
}

func memberDepartureMap(items []memberDeparture) map[string]memberDeparture {
	result := make(map[string]memberDeparture, len(items))
	for _, item := range items {
		result[item.MemberID] = item
	}
	return result
}

func adminDepartureOffset(e *core.RequestEvent) (int, error) {
	raw := e.Request.URL.Query().Get("offset")
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 || offset%50 != 0 {
		return 0, e.BadRequestError("Invalid offset.", nil)
	}
	return offset, nil
}
