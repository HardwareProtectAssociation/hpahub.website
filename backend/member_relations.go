package main

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type memberRelationSummary struct {
	AdmireCount      int    `json:"admireCount"`
	WatchCount       int    `json:"watchCount"`
	InvestigateCount int    `json:"investigateCount"`
	MyRelation       string `json:"myRelation"`
	CanRelate        bool   `json:"canRelate"`
}

func bindMemberRelationRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/members/{id}/relation", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load member.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		target, err := visibleMember(e, e.Request.PathValue("id"))
		if err != nil {
			return err
		}
		if target.Id == viewer.Id {
			return e.BadRequestError("You cannot set a relation to yourself.", nil)
		}
		var input struct {
			Relation string `json:"relation"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid relation.", err)
		}
		if input.Relation != "admire" && input.Relation != "watch" && input.Relation != "investigate" && input.Relation != "none" {
			return e.BadRequestError("Invalid relation.", nil)
		}
		existing, err := e.App.FindFirstRecordByFilter("member_relations", "actor_user_id = {:actor} && target_user_id = {:target}", dbx.Params{"actor": viewer.Id, "target": target.Id})
		if errors.Is(err, sql.ErrNoRows) {
			if input.Relation != "none" {
				collection, err := e.App.FindCollectionByNameOrId("member_relations")
				if err != nil {
					return e.InternalServerError("Unable to save relation.", err)
				}
				record := core.NewRecord(collection)
				record.Set("actor_user_id", viewer.Id)
				record.Set("target_user_id", target.Id)
				record.Set("relation", input.Relation)
				if err := e.App.Save(record); err != nil {
					return e.InternalServerError("Unable to save relation.", err)
				}
			}
		} else if err != nil {
			return e.InternalServerError("Unable to save relation.", err)
		} else if input.Relation == "none" || existing.GetString("relation") == input.Relation {
			if err := e.App.Delete(existing); err != nil {
				return e.InternalServerError("Unable to save relation.", err)
			}
			input.Relation = "none"
		} else {
			existing.Set("relation", input.Relation)
			if err := e.App.Save(existing); err != nil {
				return e.InternalServerError("Unable to save relation.", err)
			}
		}
		summary, err := memberRelationSummaryData(e.App, target.Id, viewer.Id)
		if err != nil {
			return e.InternalServerError("Unable to load relations.", err)
		}
		return e.JSON(200, summary)
	})
}

func memberRelationSummaryData(app core.App, targetID, viewerID string) (memberRelationSummary, error) {
	summary := memberRelationSummary{CanRelate: viewerID != "" && viewerID != targetID}
	relations, err := app.FindRecordsByFilter("member_relations", "target_user_id = {:target}", "", 0, 0, dbx.Params{"target": targetID})
	if err != nil {
		return summary, err
	}
	for _, relation := range relations {
		switch relation.GetString("relation") {
		case "admire":
			summary.AdmireCount++
		case "watch":
			summary.WatchCount++
		case "investigate":
			summary.InvestigateCount++
		}
		if viewerID != "" && relation.GetString("actor_user_id") == viewerID {
			summary.MyRelation = relation.GetString("relation")
		}
	}
	return summary, nil
}
