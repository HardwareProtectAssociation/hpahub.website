package main

import (
	"database/sql"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func bindMembershipApplicationRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/membership-applications", func(e *core.RequestEvent) error {
		noCache(e)
		allowed, err := allowAuthAttempt(e.App, "membership-application", 10)
		if err != nil {
			return e.InternalServerError("Unable to check application rate.", err)
		}
		if !allowed {
			return e.TooManyRequestsError("提交过于频繁，请稍后再试。", nil)
		}
		var input struct {
			Email  string `json:"email"`
			Name   string `json:"name"`
			Reason string `json:"reason"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("请填写有效的申请信息。", nil)
		}
		input.Email = strings.ToLower(strings.TrimSpace(input.Email))
		input.Name = strings.TrimSpace(input.Name)
		input.Reason = strings.TrimSpace(input.Reason)
		parsed, parseErr := mail.ParseAddress(input.Email)
		if parseErr != nil || parsed.Address != input.Email || len([]rune(input.Name)) == 0 || len([]rune(input.Name)) > 80 || len([]rune(input.Reason)) == 0 || len([]rune(input.Reason)) > 2000 {
			return e.BadRequestError("请填写有效邮箱、昵称和申请说明。", nil)
		}
		if _, err := e.App.FindAuthRecordByEmail("users", input.Email); err == nil {
			return e.BadRequestError("此邮箱已有账号，或申请已提交；如需帮助请联系管理员。", nil)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return e.InternalServerError("Unable to check applicant.", err)
		}
		if _, err := e.App.FindAuthRecordByEmail(core.CollectionNameSuperusers, input.Email); err == nil {
			return e.BadRequestError("此邮箱已有账号，或申请已提交；如需帮助请联系管理员。", nil)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return e.InternalServerError("Unable to check applicant.", err)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			collection, err := tx.FindCollectionByNameOrId("membership_applications")
			if err != nil {
				return err
			}
			application := core.NewRecord(collection)
			application.Set("email", input.Email)
			application.Set("applicant_name", input.Name)
			application.Set("reason", input.Reason)
			application.Set("status", "pending")
			return tx.Save(application)
		})
		if err != nil {
			return e.BadRequestError("此邮箱已有待处理申请，或申请无法提交。", nil)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})

	e.Router.GET("/api/hpahub/manage/membership-applications", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		offset, err := adminOffset(e)
		if err != nil {
			return err
		}
		rows, err := e.App.FindRecordsByFilter("membership_applications", "", "-created,id", 101, offset)
		if err != nil {
			return e.InternalServerError("Unable to load membership applications.", err)
		}
		hasMore := len(rows) > 100
		if hasMore {
			rows = rows[:100]
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			item := map[string]any{
				"id": row.Id, "email": row.GetString("email"), "name": row.GetString("applicant_name"), "reason": row.GetString("reason"),
				"status": row.GetString("status"), "created": row.GetString("created"), "reviewedAt": row.GetString("reviewed_at"),
			}
			if row.GetString("status") == "approved" {
				item["invitationCode"] = row.GetString("invitation_code")
				if expiry := row.GetInt("invitation_expires"); expiry > 0 {
					item["invitationExpiresAt"] = time.Unix(int64(expiry), 0).UTC().Format(time.RFC3339)
				}
			}
			items = append(items, item)
		}
		return e.JSON(200, map[string]any{"items": items, "hasMore": hasMore})
	})

	e.Router.PATCH("/api/hpahub/manage/membership-applications/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if err := requireConsoleAdmin(e); err != nil {
			return err
		}
		var input struct {
			Status string `json:"status"`
		}
		if err := e.BindBody(&input); err != nil || (input.Status != "approved" && input.Status != "rejected") {
			return e.BadRequestError("Invalid application status.", nil)
		}
		now := time.Now().UTC()
		var invitation invitation
		err := e.App.RunInTransaction(func(tx core.App) error {
			application, err := tx.FindRecordById("membership_applications", e.Request.PathValue("id"))
			if err != nil {
				return e.NotFoundError("Application not found.", nil)
			}
			if application.GetString("status") != "pending" {
				return e.BadRequestError("Application already handled.", nil)
			}
			if input.Status == "approved" {
				email := application.GetString("email")
				if _, err := tx.FindAuthRecordByEmail("users", email); err == nil {
					return e.BadRequestError("该邮箱已注册，无法继续发放邀请码。", nil)
				} else if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				invitation, err = newInvitationWithUses(tx, now, e.Auth.Id, 1, 24*time.Hour)
				if err != nil {
					return err
				}
				application.Set("invitation_code", invitation.Code)
				application.Set("invitation_expires", invitation.Expires)
			}
			application.Set("status", input.Status)
			application.Set("reviewer_user_id", e.Auth.Id)
			application.Set("reviewed_at", now)
			return tx.Save(application)
		})
		if err != nil {
			return e.BadRequestError("Unable to review application.", err)
		}
		result := map[string]any{"ok": true, "status": input.Status}
		if input.Status == "approved" {
			result["invitationCode"] = invitation.Code
			result["invitationExpiresAt"] = time.Unix(invitation.Expires, 0).UTC().Format(time.RFC3339)
		}
		return e.JSON(200, result)
	})

}
