package main

import (
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type homeProject struct {
	publicProject
	FollowCount int `json:"followCount"`
}

type homeEngineer struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	AvatarURL   string           `json:"avatarUrl"`
	Identities  []map[string]any `json:"identities"`
	AdmireCount int              `json:"admireCount"`
}

// bindHomeRoutes serves the home feed: hot projects ranked by active follows
// and hot engineers ranked by admire relations. Both lists are capped at 8.
func bindHomeRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/home", func(e *core.RequestEvent) error {
		noCache(e)
		projects, err := homeHotProjects(e)
		if err != nil {
			return e.InternalServerError("Unable to load home feed.", err)
		}
		engineers, err := homeHotEngineers(e)
		if err != nil {
			return e.InternalServerError("Unable to load home feed.", err)
		}
		return e.JSON(200, map[string]any{"popularProjects": projects, "popularEngineers": engineers})
	})
}

func homeHotProjects(e *core.RequestEvent) ([]homeProject, error) {
	var rows []*core.Record
	err := e.App.RecordQuery("projects").
		AndWhere(dbx.HashExp{"review_status": "approved", "deleted_at": ""}).
		OrderBy("(SELECT COUNT(*) FROM project_follows WHERE project_follows.project_id = projects.id AND is_active = 1) DESC", "updated DESC", "id DESC").
		Limit(8).All(&rows)
	if err != nil {
		return nil, err
	}
	followCounts, err := projectFollowCounts(e.App, rows)
	if err != nil {
		return nil, err
	}
	owners := map[string]*core.Record{}
	items := make([]homeProject, 0, len(rows))
	for _, project := range rows {
		if project.GetString("deleted_at") != "" {
			continue
		}
		items = append(items, homeProject{publicProject: projectSummary(e, project, owners), FollowCount: followCounts[project.Id]})
	}
	return items, nil
}

func homeHotEngineers(e *core.RequestEvent) ([]homeEngineer, error) {
	users, err := e.App.FindRecordsByFilter("users", `role = "member" || role = "admin"`, "-created", 0, 0)
	if err != nil {
		return nil, err
	}
	relations, err := e.App.FindRecordsByFilter("member_relations", `relation = "admire"`, "", 0, 0)
	if err != nil {
		return nil, err
	}
	admireCounts := map[string]int{}
	for _, relation := range relations {
		admireCounts[relation.GetString("target_user_id")]++
	}
	type candidate struct {
		item   homeEngineer
		record *core.Record
	}
	candidates := []candidate{}
	for _, member := range users {
		if !member.GetBool("profilePublic") {
			continue
		}
		avatarURL := ""
		if member.GetString("avatar") != "" && !profileSectionHidden(member, "avatar") {
			avatarURL = contentURL("members", member.Id, "avatar")
		}
		candidates = append(candidates, candidate{
			item: homeEngineer{
				ID:          member.Id,
				Name:        publicPersonName(member),
				AvatarURL:   avatarURL,
				Identities:  []map[string]any{},
				AdmireCount: admireCounts[member.Id],
			},
			record: member,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].item.AdmireCount > candidates[j].item.AdmireCount })
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	items := make([]homeEngineer, 0, len(candidates))
	for _, entry := range candidates {
		identities, err := publicMemberIdentities(e, entry.record)
		if err != nil {
			return nil, err
		}
		entry.item.Identities = identities
		items = append(items, entry.item)
	}
	return items, nil
}
