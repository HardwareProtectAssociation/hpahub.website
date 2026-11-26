package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type hardwareNewsItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Link         string `json:"link"`
	AuthorID     string `json:"authorId"`
	AuthorName   string `json:"authorName"`
	Created      string `json:"created"`
	LikeCount    int    `json:"likeCount"`
	UselessCount int    `json:"uselessCount"`
	MyReaction   string `json:"myReaction"`
	CanDelete    bool   `json:"canDelete"`
	IsAuthor     bool   `json:"isAuthor"`
}

func bindNewsRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/news", func(e *core.RequestEvent) error {
		noCache(e)
		rows, err := e.App.FindRecordsByFilter("hardware_news", "", "-created", 100, 0)
		if err != nil {
			return e.InternalServerError("Unable to load news.", err)
		}
		var reactions []*core.Record
		if len(rows) > 0 {
			filters := make([]string, 0, len(rows))
			params := dbx.Params{}
			for i, row := range rows {
				key := fmt.Sprintf("news%d", i)
				filters = append(filters, "news_id = {:"+key+"}")
				params[key] = row.Id
			}
			reactions, err = e.App.FindRecordsByFilter("hardware_news_reactions", strings.Join(filters, " || "), "", 0, 0, params)
			if err != nil {
				return e.InternalServerError("Unable to load news.", err)
			}
		}
		viewerID := ""
		role := ""
		if e.Auth != nil {
			role = siteRole(e.Auth)
			if owner, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member {
				viewerID = owner.Id
			}
		}
		likes := map[string]int{}
		useless := map[string]int{}
		mine := map[string]string{}
		for _, reaction := range reactions {
			newsID := reaction.GetString("news_id")
			switch reaction.GetString("reaction") {
			case "like":
				likes[newsID]++
			case "useless":
				useless[newsID]++
			}
			if viewerID != "" && reaction.GetString("user_id") == viewerID {
				mine[newsID] = reaction.GetString("reaction")
			}
		}
		items := make([]hardwareNewsItem, 0, len(rows))
		for _, row := range rows {
			authorName := ""
			authorID := row.GetString("author_user_id")
			if author, err := e.App.FindRecordById("users", authorID); err == nil {
				authorName = publicPersonName(author)
			}
			items = append(items, hardwareNewsItem{
				ID:           row.Id,
				Title:        row.GetString("title"),
				Content:      row.GetString("content"),
				Link:         row.GetString("link"),
				AuthorID:     authorID,
				AuthorName:   authorName,
				Created:      row.GetString("created"),
				LikeCount:    likes[row.Id],
				UselessCount: useless[row.Id],
				MyReaction:   mine[row.Id],
				CanDelete:    viewerID != "" && (viewerID == authorID || role == "admin" || role == "octopus"),
				IsAuthor:     viewerID != "" && viewerID == authorID,
			})
		}
		return e.JSON(200, map[string]any{"items": items})
	})

	e.Router.POST("/api/hpahub/news", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := newsOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Link    string `json:"link"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid news.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Content = strings.TrimSpace(input.Content)
		input.Link = strings.TrimSpace(input.Link)
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || input.Content == "" || utf8.RuneCountInString(input.Content) > 5000 || !validNewsLink(input.Link) {
			return e.BadRequestError("Invalid news.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("hardware_news")
		if err != nil {
			return e.InternalServerError("Unable to publish news.", err)
		}
		record := core.NewRecord(collection)
		record.Set("author_user_id", owner.Id)
		record.Set("title", input.Title)
		record.Set("content", input.Content)
		record.Set("link", input.Link)
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Unable to publish news.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "id": record.Id})
	})

	e.Router.DELETE("/api/hpahub/news/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := newsOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		record, err := e.App.FindRecordById("hardware_news", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		role := siteRole(e.Auth)
		if record.GetString("author_user_id") != owner.Id && role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		if err := e.App.Delete(record); err != nil {
			return e.InternalServerError("Unable to delete news.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})

	e.Router.POST("/api/hpahub/news/{id}/reaction", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := newsOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		record, err := e.App.FindRecordById("hardware_news", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		if record.GetString("author_user_id") == owner.Id {
			return e.BadRequestError("You cannot react to your own news.", nil)
		}
		var input struct {
			Reaction string `json:"reaction"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid reaction.", err)
		}
		if input.Reaction != "like" && input.Reaction != "useless" && input.Reaction != "none" {
			return e.BadRequestError("Invalid reaction.", nil)
		}
		reaction, err := e.App.FindFirstRecordByFilter("hardware_news_reactions", "news_id = {:news} && user_id = {:user}", dbx.Params{"news": record.Id, "user": owner.Id})
		if errors.Is(err, sql.ErrNoRows) {
			if input.Reaction != "none" {
				collection, err := e.App.FindCollectionByNameOrId("hardware_news_reactions")
				if err != nil {
					return e.InternalServerError("Unable to save reaction.", err)
				}
				reaction = core.NewRecord(collection)
				reaction.Set("news_id", record.Id)
				reaction.Set("user_id", owner.Id)
				reaction.Set("reaction", input.Reaction)
				if err := e.App.Save(reaction); err != nil {
					return e.InternalServerError("Unable to save reaction.", err)
				}
			}
		} else if err != nil {
			return e.InternalServerError("Unable to save reaction.", err)
		} else if input.Reaction == "none" {
			if err := e.App.Delete(reaction); err != nil {
				return e.InternalServerError("Unable to save reaction.", err)
			}
		} else {
			reaction.Set("reaction", input.Reaction)
			if err := e.App.Save(reaction); err != nil {
				return e.InternalServerError("Unable to save reaction.", err)
			}
		}
		likes := 0
		useless := 0
		all, err := e.App.FindRecordsByFilter("hardware_news_reactions", "news_id = {:news}", "", 0, 0, dbx.Params{"news": record.Id})
		if err != nil {
			return e.InternalServerError("Unable to load reaction.", err)
		}
		for _, item := range all {
			if item.GetString("reaction") == "like" {
				likes++
			} else if item.GetString("reaction") == "useless" {
				useless++
			}
		}
		return e.JSON(200, map[string]any{
			"likeCount":    likes,
			"uselessCount": useless,
			"myReaction":   input.Reaction,
		})
	})
}

func newsOwner(e *core.RequestEvent) (*core.Record, bool, error) {
	if e.Auth == nil {
		return nil, false, e.UnauthorizedError("Authentication required.", nil)
	}
	owner, member, err := personalCenterOwner(e.App, e.Auth)
	if err != nil {
		return nil, false, e.InternalServerError("Unable to load news.", err)
	}
	return owner, member, nil
}

func validNewsLink(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") && utf8.RuneCountInString(value) <= 500
}
