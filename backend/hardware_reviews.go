package main

import (
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type hardwareReview struct {
	ID         string `json:"id"`
	HardwareID string `json:"hardwareId"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	AuthorID   string `json:"authorId"`
	AuthorName string `json:"authorName"`
	Created    string `json:"created"`
	LikeCount  int    `json:"likeCount"`
	LikedByMe  bool   `json:"likedByMe"`
	CanLike    bool   `json:"canLike"`
	CanDelete  bool   `json:"canDelete"`
}

// bindHardwareReviewRoutes serves member reviews of hardware items with one
// like per member per review; home page hot hardware ranks by these likes.
func bindHardwareReviewRoutes(e *core.ServeEvent) {
	base := "/api/hpahub/hardware"
	e.Router.GET(base+"/records/{id}/reviews", func(e *core.RequestEvent) error {
		noCache(e)
		if _, err := e.App.FindRecordById("hardware", e.Request.PathValue("id")); err != nil {
			return e.NotFoundError("Hardware not found.", nil)
		}
		reviews, err := hardwareReviewList(e, e.Request.PathValue("id"))
		if err != nil {
			return e.InternalServerError("Unable to load reviews.", err)
		}
		return e.JSON(200, map[string]any{"items": reviews})
	})
	e.Router.POST(base+"/records/{id}/reviews", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		author, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to create review.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		if _, err := e.App.FindRecordById("hardware", e.Request.PathValue("id")); err != nil {
			return e.NotFoundError("Hardware not found.", nil)
		}
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid review.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Content = strings.TrimSpace(input.Content)
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || input.Content == "" || utf8.RuneCountInString(input.Content) > 5000 {
			return e.BadRequestError("Invalid review.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("hardware_reviews")
		if err != nil {
			return e.InternalServerError("Unable to create review.", err)
		}
		review := core.NewRecord(collection)
		review.Set("hardware_id", e.Request.PathValue("id"))
		review.Set("author_user_id", author.Id)
		review.Set("title", input.Title)
		review.Set("content", input.Content)
		if err := e.App.Save(review); err != nil {
			return e.InternalServerError("Unable to create review.", err)
		}
		return e.JSON(200, hardwareReviewData(e.App, review, author.Id, siteRole(e.Auth)))
	})
	e.Router.POST(base+"/reviews/{id}/like", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to like review.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		review, err := e.App.FindRecordById("hardware_reviews", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		if review.GetString("author_user_id") == viewer.Id {
			return e.BadRequestError("You cannot like your own review.", nil)
		}
		like, err := e.App.FindFirstRecordByFilter("hardware_review_likes", "review_id = {:review} && user_id = {:user}", dbx.Params{"review": review.Id, "user": viewer.Id})
		if errors.Is(err, sql.ErrNoRows) {
			collection, err := e.App.FindCollectionByNameOrId("hardware_review_likes")
			if err != nil {
				return e.InternalServerError("Unable to like review.", err)
			}
			like = core.NewRecord(collection)
			like.Set("review_id", review.Id)
			like.Set("user_id", viewer.Id)
		} else if err != nil {
			return e.InternalServerError("Unable to like review.", err)
		} else if err := e.App.Delete(like); err != nil {
			return e.InternalServerError("Unable to unlike review.", err)
		} else {
			count, err := hardwareReviewLikeCount(e.App, review.Id)
			if err != nil {
				return e.InternalServerError("Unable to like review.", err)
			}
			return e.JSON(200, map[string]any{"liked": false, "likeCount": count})
		}
		if err := e.App.Save(like); err != nil {
			return e.InternalServerError("Unable to like review.", err)
		}
		count, err := hardwareReviewLikeCount(e.App, review.Id)
		if err != nil {
			return e.InternalServerError("Unable to like review.", err)
		}
		return e.JSON(200, map[string]any{"liked": true, "likeCount": count})
	})
	e.Router.DELETE(base+"/reviews/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		review, err := e.App.FindRecordById("hardware_reviews", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		role := siteRole(e.Auth)
		if review.GetString("author_user_id") != e.Auth.Id && role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		if err := e.App.Delete(review); err != nil {
			return e.InternalServerError("Unable to delete review.", err)
		}
		return e.NoContent(204)
	})
}

func hardwareReviewList(e *core.RequestEvent, hardwareID string) ([]hardwareReview, error) {
	viewerID := ""
	role := ""
	if e.Auth != nil {
		role = siteRole(e.Auth)
		if owner, ok, err := personalCenterOwner(e.App, e.Auth); err == nil && ok {
			viewerID = owner.Id
		}
	}
	rows, err := e.App.FindRecordsByFilter("hardware_reviews", "hardware_id = {:hardware}", "-created,-id", 0, 0, dbx.Params{"hardware": hardwareID})
	if err != nil {
		return nil, err
	}
	reviews := make([]hardwareReview, 0, len(rows))
	for _, row := range rows {
		review := hardwareReviewData(e.App, row, viewerID, role)
		reviews = append(reviews, review)
	}
	return reviews, nil
}

func hardwareReviewData(app core.App, row *core.Record, viewerID, role string) hardwareReview {
	likes, _ := app.FindRecordsByFilter("hardware_review_likes", "review_id = {:review}", "", 0, 0, dbx.Params{"review": row.Id})
	liked := false
	for _, like := range likes {
		if like.GetString("user_id") == viewerID {
			liked = true
		}
	}
	authorID := row.GetString("author_user_id")
	authorName := ""
	if author, err := app.FindRecordById("users", authorID); err == nil {
		authorName = publicPersonName(author)
	}
	return hardwareReview{
		ID:         row.Id,
		HardwareID: row.GetString("hardware_id"),
		Title:      row.GetString("title"),
		Content:    row.GetString("content"),
		AuthorID:   authorID,
		AuthorName: authorName,
		Created:    row.GetString("created"),
		LikeCount:  len(likes),
		LikedByMe:  liked,
		CanLike:    viewerID != "" && viewerID != authorID,
		CanDelete:  viewerID != "" && (viewerID == authorID || role == "admin" || role == "octopus"),
	}
}

func hardwareReviewLikeCount(app core.App, reviewID string) (int, error) {
	likes, err := app.FindRecordsByFilter("hardware_review_likes", "review_id = {:review}", "", 0, 0, dbx.Params{"review": reviewID})
	if err != nil {
		return 0, err
	}
	return len(likes), nil
}
