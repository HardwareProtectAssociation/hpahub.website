package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type messageItem struct {
	ID           string `json:"id"`
	SenderID     string `json:"senderId"`
	SenderName   string `json:"senderName"`
	RecipientID  string `json:"recipientId"`
	Subject      string `json:"subject"`
	Content      string `json:"content"`
	Category     string `json:"category"`
	ProjectID    string `json:"projectId,omitempty"`
	ProjectTitle string `json:"projectTitle,omitempty"`
	Read         bool   `json:"read"`
	Created      string `json:"created"`
}

type messageList struct {
	Items       []messageItem `json:"items"`
	UnreadCount int           `json:"unreadCount"`
}

func bindMessageRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/me/messages", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := messageOwner(e)
		if err != nil {
			return err
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		limit := 50
		offset := 0
		if e.Request.URL.Query().Get("limit") != "" {
			fmt.Sscanf(e.Request.URL.Query().Get("limit"), "%d", &limit)
		}
		if e.Request.URL.Query().Get("offset") != "" {
			fmt.Sscanf(e.Request.URL.Query().Get("offset"), "%d", &offset)
		}
		if limit > 100 {
			limit = 100
		}
		messages, err := e.App.FindRecordsByFilter("messages",
			"recipient_user_id = {:user}", "-created,-id", limit, offset, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load messages.", err)
		}
		unread, err := e.App.FindRecordsByFilter("messages",
			"recipient_user_id = {:user} && read = false", "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load messages.", err)
		}
		items := make([]messageItem, 0, len(messages))
		for _, msg := range messages {
			item := messageToItem(e.App, msg)
			items = append(items, item)
		}
		return e.JSON(200, messageList{Items: items, UnreadCount: len(unread)})
	})

	e.Router.POST("/api/hpahub/me/messages", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		sender, ok, err := messageOwner(e)
		if err != nil {
			return err
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			RecipientID string `json:"recipientId"`
			Subject     string `json:"subject"`
			Content     string `json:"content"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid message.", err)
		}
		input.Subject = strings.TrimSpace(input.Subject)
		input.Content = strings.TrimSpace(input.Content)
		if input.RecipientID == "" || input.Content == "" || utf8.RuneCountInString(input.Content) > 5000 || utf8.RuneCountInString(input.Subject) > 200 {
			return e.BadRequestError("Invalid message.", nil)
		}
		if input.RecipientID == sender.Id {
			return e.BadRequestError("Cannot send message to yourself.", nil)
		}
		recipient, err := e.App.FindRecordById("users", input.RecipientID)
		if err != nil {
			return e.NotFoundError("Recipient not found.", nil)
		}
		// Verify recipient is a visible member
		if !recipient.GetBool("profilePublic") && siteRole(recipient) == "user" {
			return e.NotFoundError("Recipient not found.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("messages")
		if err != nil {
			return e.InternalServerError("Unable to send message.", err)
		}
		msg := core.NewRecord(collection)
		msg.Set("sender_user_id", sender.Id)
		msg.Set("recipient_user_id", recipient.Id)
		msg.Set("subject", input.Subject)
		msg.Set("content", input.Content)
		msg.Set("category", "user")
		msg.Set("read", false)
		if err := e.App.Save(msg); err != nil {
			return e.InternalServerError("Unable to send message.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "id": msg.Id})
	})

	e.Router.POST("/api/hpahub/me/messages/{id}/read", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := messageOwner(e)
		if err != nil {
			return err
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		msg, err := e.App.FindRecordById("messages", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		if msg.GetString("recipient_user_id") != member.Id {
			return e.ForbiddenError("Not your message.", nil)
		}
		msg.Set("read", true)
		msg.Set("read_at", time.Now())
		if err := e.App.Save(msg); err != nil {
			return e.InternalServerError("Unable to mark as read.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})

	e.Router.POST("/api/hpahub/me/messages/read-all", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := messageOwner(e)
		if err != nil {
			return err
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		unread, err := e.App.FindRecordsByFilter("messages",
			"recipient_user_id = {:user} && read = false", "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to mark as read.", err)
		}
		now := time.Now()
		for _, msg := range unread {
			msg.Set("read", true)
			msg.Set("read_at", now)
			if err := e.App.Save(msg); err != nil {
				return e.InternalServerError("Unable to mark as read.", err)
			}
		}
		return e.JSON(200, map[string]any{"ok": true, "count": len(unread)})
	})

	e.Router.GET("/api/hpahub/me/messages/unread-count", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := messageOwner(e)
		if err != nil {
			return err
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		unread, err := e.App.FindRecordsByFilter("messages",
			"recipient_user_id = {:user} && read = false", "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load unread count.", err)
		}
		return e.JSON(200, map[string]any{"unreadCount": len(unread)})
	})
}

func messageOwner(e *core.RequestEvent) (*core.Record, bool, error) {
	if e.Auth == nil {
		return nil, false, e.UnauthorizedError("Authentication required.", nil)
	}
	owner, member, err := personalCenterOwner(e.App, e.Auth)
	if err != nil {
		return nil, false, e.InternalServerError("Unable to load messages.", err)
	}
	return owner, member, nil
}

func messageToItem(app core.App, msg *core.Record) messageItem {
	item := messageItem{
		ID:          msg.Id,
		SenderID:    msg.GetString("sender_user_id"),
		RecipientID: msg.GetString("recipient_user_id"),
		Subject:     msg.GetString("subject"),
		Content:     msg.GetString("content"),
		Category:    msg.GetString("category"),
		ProjectID:   msg.GetString("project_id"),
		Read:        msg.GetBool("read"),
		Created:     msg.GetString("created"),
	}
	if item.SenderID != "" {
		if sender, err := app.FindRecordById("users", item.SenderID); err == nil {
			item.SenderName = publicPersonName(sender)
		}
	} else {
		item.SenderName = "系统"
	}
	if item.ProjectID != "" {
		if project, err := app.FindRecordById("projects", item.ProjectID); err == nil {
			item.ProjectTitle = project.GetString("title")
		}
	}
	return item
}

// sendSystemMessage sends a system notification to a user.
func sendSystemMessage(app core.App, recipientID, subject, content, category, projectID string) error {
	collection, err := app.FindCollectionByNameOrId("messages")
	if err != nil {
		return err
	}
	msg := core.NewRecord(collection)
	msg.Set("recipient_user_id", recipientID)
	msg.Set("subject", subject)
	msg.Set("content", content)
	msg.Set("category", category)
	if projectID != "" {
		msg.Set("project_id", projectID)
	}
	msg.Set("read", false)
	return app.Save(msg)
}
