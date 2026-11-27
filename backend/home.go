package main

import (
	"sort"

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

type homeHardware struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Brand     string `json:"brand"`
	Subtitle  string `json:"subtitle"`
	Kind      string `json:"kind"`
	LikeCount int    `json:"likeCount"`
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
		hardware, err := homeHotHardware(e)
		if err != nil {
			return e.InternalServerError("Unable to load home feed.", err)
		}
		return e.JSON(200, map[string]any{"popularProjects": projects, "popularEngineers": engineers, "popularHardware": hardware})
	})
}

func homeHotProjects(e *core.RequestEvent) ([]homeProject, error) {
	rows, err := e.App.FindRecordsByFilter("projects", `review_status = "approved"`, "-updated", 100, 0)
	if err != nil {
		return nil, err
	}
	follows, err := e.App.FindRecordsByFilter("project_follows", "is_active = true", "", 0, 0)
	if err != nil {
		return nil, err
	}
	followCounts := map[string]int{}
	for _, follow := range follows {
		followCounts[follow.GetString("project_id")]++
	}
	owners := map[string]*core.Record{}
	items := make([]homeProject, 0, len(rows))
	for _, project := range rows {
		if project.GetString("deleted_at") != "" {
			continue
		}
		items = append(items, homeProject{publicProject: projectSummary(e, project, owners), FollowCount: followCounts[project.Id]})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].FollowCount != items[j].FollowCount {
			return items[i].FollowCount > items[j].FollowCount
		}
		return items[i].Updated > items[j].Updated
	})
	if len(items) > 8 {
		items = items[:8]
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

// homeHotHardware ranks hardware by likes on its member reviews; ties keep the
// curated sort_order.
func homeHotHardware(e *core.RequestEvent) ([]homeHardware, error) {
	rows, err := e.App.FindRecordsByFilter("hardware", "", "sort_order,id", 100, 0)
	if err != nil {
		return nil, err
	}
	var totals []struct {
		HardwareID string `db:"hardware_id"`
		Total      int    `db:"total"`
	}
	if err := e.App.DB().NewQuery(`SELECT reviews.hardware_id AS hardware_id, COUNT(likes.id) AS total FROM hardware_review_likes AS likes INNER JOIN hardware_reviews AS reviews ON reviews.id = likes.review_id GROUP BY reviews.hardware_id`).All(&totals); err != nil {
		return nil, err
	}
	likeCounts := map[string]int{}
	for _, row := range totals {
		likeCounts[row.HardwareID] = row.Total
	}
	items := make([]homeHardware, 0, len(rows))
	for _, record := range rows {
		items = append(items, homeHardware{
			ID:        record.Id,
			Name:      record.GetString("name"),
			Brand:     record.GetString("brand"),
			Subtitle:  record.GetString("subtitle"),
			Kind:      record.GetString("kind"),
			LikeCount: likeCounts[record.Id],
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].LikeCount > items[j].LikeCount })
	if len(items) > 8 {
		items = items[:8]
	}
	return items, nil
}
