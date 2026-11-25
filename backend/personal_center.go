package main

import (
	"database/sql"
	"errors"
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type centerProfile struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Bio           string `json:"bio"`
	AvatarURL     string `json:"avatarUrl"`
	ProfilePublic bool   `json:"profilePublic"`
	Role          string `json:"role"`
	Member        bool   `json:"member"`
}

type centerIdentity struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type centerSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Proficiency string `json:"proficiency"`
	AbilityNote string `json:"abilityNote"`
}

type centerPreferences struct {
	Willingness         string   `json:"willingness"`
	HelpTypes           []string `json:"helpTypes"`
	HelpDescription     string   `json:"helpDescription"`
	WeeklyHours         float64  `json:"weeklyHours"`
	AvailabilityNote    string   `json:"availabilityNote"`
	ExpectedRewardTypes []string `json:"expectedRewardTypes"`
	ExpectedRewardNote  string   `json:"expectedRewardNote"`
	Visibility          string   `json:"visibility"`
}

type centerProject struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	ReviewStatus   string `json:"reviewStatus"`
	AdventureStyle string `json:"adventureStyle"`
	Relation       string `json:"relation"`
	Duties         string `json:"duties"`
	Updated        string `json:"updated"`
}

type centerBadge struct {
	CareerID       string `json:"careerId"`
	CareerName     string `json:"careerName"`
	Category       string `json:"category"`
	Level          int    `json:"level"`
	LevelTitle     string `json:"levelTitle"`
	LevelMinXP     int64  `json:"levelMinXp"`
	TotalXP        int64  `json:"totalXp"`
	NextLevelXP    int64  `json:"nextLevelXp,omitempty"`
	NextLevelTitle string `json:"nextLevelTitle,omitempty"`
}

type centerProjects struct {
	Participating []centerProject `json:"participating"`
	Completed     []centerProject `json:"completed"`
}

type centerResponse struct {
	Profile     centerProfile      `json:"profile"`
	Identities  []centerIdentity   `json:"identities"`
	Projects    centerProjects     `json:"projects"`
	Badges      []centerBadge      `json:"badges"`
	Skills      []centerSkill      `json:"skills"`
	Preferences *centerPreferences `json:"preferences"`
}

// bindPersonalCenterRoutes exposes the signed-in member's own summary. It is
// not a public profile endpoint: it returns the caller's email and account
// details and requires a valid session.
func bindPersonalCenterRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/me/center", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		data, err := personalCenterData(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load personal center.", err)
		}
		return e.JSON(200, data)
	})
}

func personalCenterData(app core.App, auth *core.Record) (centerResponse, error) {
	member := auth.Collection().Name == "users"
	response := centerResponse{
		Profile: centerProfile{
			ID:     auth.Id,
			Email:  auth.Email(),
			Role:   siteRole(auth),
			Member: member,
		},
		Identities: []centerIdentity{},
		Projects: centerProjects{
			Participating: []centerProject{},
			Completed:     []centerProject{},
		},
		Badges: []centerBadge{},
		Skills: []centerSkill{},
	}
	if !member {
		return response, nil
	}

	response.Profile.Name = auth.GetString("display_name")
	if response.Profile.Name == "" {
		response.Profile.Name = auth.GetString("name")
	}
	if response.Profile.Name == "" {
		response.Profile.Name = auth.Email()
	}
	response.Profile.Bio = auth.GetString("bio")
	response.Profile.ProfilePublic = auth.GetBool("profilePublic")
	if auth.GetString("avatar") != "" {
		response.Profile.AvatarURL = contentURL("me", "profile", "avatar")
	}

	identities, err := centerIdentityRecords(app)
	if err != nil {
		return response, err
	}
	identityRows, err := app.FindRecordsByFilter("user_identities", "user_id = {:user}", "", 0, 0, dbx.Params{"user": auth.Id})
	if err != nil {
		return response, err
	}
	for _, row := range identityRows {
		identity := identities[row.GetString("identity_id")]
		if identity == nil {
			continue
		}
		response.Identities = append(response.Identities, centerIdentity{
			ID:       identity.Id,
			Name:     identity.GetString("name"),
			Category: identity.GetString("category"),
		})
	}

	skills, err := centerSkillRecords(app)
	if err != nil {
		return response, err
	}
	skillRows, err := app.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": auth.Id})
	if err != nil {
		return response, err
	}
	for _, row := range skillRows {
		skill := skills[row.GetString("skill_id")]
		if skill == nil {
			continue
		}
		response.Skills = append(response.Skills, centerSkill{
			ID:          skill.Id,
			Name:        skill.GetString("name"),
			Category:    skill.GetString("category"),
			Proficiency: row.GetString("proficiency"),
			AbilityNote: row.GetString("ability_note"),
		})
	}

	preferences, err := app.FindFirstRecordByFilter(
		"user_collaboration_preferences",
		"user_id = {:user}",
		dbx.Params{"user": auth.Id},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return response, err
	}
	if preferences != nil {
		response.Preferences = &centerPreferences{
			Willingness:         preferences.GetString("willingness"),
			HelpTypes:           centerStrings(preferences.GetStringSlice("help_types")),
			HelpDescription:     preferences.GetString("help_description"),
			WeeklyHours:         preferences.GetFloat("weekly_hours"),
			AvailabilityNote:    preferences.GetString("availability_note"),
			ExpectedRewardTypes: centerStrings(preferences.GetStringSlice("expected_reward_types")),
			ExpectedRewardNote:  preferences.GetString("expected_reward_note"),
			Visibility:          preferences.GetString("visibility"),
		}
	}

	seen := map[string]bool{}
	addProject := func(project *core.Record, relation, duties string) {
		if project == nil || seen[project.Id] || project.GetString("deleted_at") != "" {
			return
		}
		seen[project.Id] = true
		item := centerProject{
			ID:             project.Id,
			Title:          project.GetString("title"),
			Summary:        project.GetString("summary"),
			Status:         project.GetString("execution_status"),
			ReviewStatus:   project.GetString("review_status"),
			AdventureStyle: project.GetString("adventure_style"),
			Relation:       relation,
			Duties:         duties,
			Updated:        project.GetString("updated"),
		}
		if relation == "owner" && item.Status == "completed" {
			response.Projects.Completed = append(response.Projects.Completed, item)
			return
		}
		response.Projects.Participating = append(response.Projects.Participating, item)
	}

	owned, err := app.FindRecordsByFilter("projects", "owner_user_id = {:user}", "-updated", 0, 0, dbx.Params{"user": auth.Id})
	if err != nil {
		return response, err
	}
	for _, project := range owned {
		addProject(project, "owner", "")
	}
	memberships, err := app.FindRecordsByFilter("project_members", `user_id = {:user} && status = "active"`, "", 0, 0, dbx.Params{"user": auth.Id})
	if err != nil {
		return response, err
	}
	for _, membership := range memberships {
		project, err := app.FindRecordById("projects", membership.GetString("project_id"))
		if err != nil {
			continue // removed project or stale membership
		}
		relation := "member"
		if project.GetString("owner_user_id") == auth.Id {
			relation = "owner"
		}
		addProject(project, relation, membership.GetString("duties"))
	}
	sortCenterProjects(response.Projects.Participating)
	sortCenterProjects(response.Projects.Completed)
	sort.SliceStable(response.Identities, func(i, j int) bool {
		if response.Identities[i].Category != response.Identities[j].Category {
			return response.Identities[i].Category < response.Identities[j].Category
		}
		return response.Identities[i].Name < response.Identities[j].Name
	})
	sort.SliceStable(response.Skills, func(i, j int) bool { return response.Skills[i].Name < response.Skills[j].Name })

	badges, err := centerBadges(app, auth.Id, identities)
	if err != nil {
		return response, err
	}
	response.Badges = badges
	return response, nil
}

func centerIdentityRecords(app core.App) (map[string]*core.Record, error) {
	rows, err := app.FindRecordsByFilter("collaboration_identities", "", "category,name", 0, 0)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*core.Record, len(rows))
	for _, row := range rows {
		result[row.Id] = row
	}
	return result, nil
}

func centerSkillRecords(app core.App) (map[string]*core.Record, error) {
	rows, err := app.FindRecordsByFilter("skills", "", "name", 0, 0)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*core.Record, len(rows))
	for _, row := range rows {
		result[row.Id] = row
	}
	return result, nil
}

func centerStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func sortCenterProjects(items []centerProject) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Updated > items[j].Updated })
}

type centerXPTotal struct {
	CareerIdentityID string `db:"career_identity_id"`
	Total            int64  `db:"total"`
}

func centerBadges(app core.App, userID string, identities map[string]*core.Record) ([]centerBadge, error) {
	badges := []centerBadge{}
	var totals []centerXPTotal
	err := app.DB().NewQuery(
		`SELECT career_identity_id, COALESCE(SUM(xp_delta), 0) AS total
		 FROM career_xp_ledger WHERE user_id = {:user} GROUP BY career_identity_id`,
	).Bind(dbx.Params{"user": userID}).All(&totals)
	if err != nil {
		return nil, err
	}
	if len(totals) == 0 {
		return badges, nil
	}
	thresholds, err := app.FindRecordsByFilter("career_level_thresholds", "", "level_number", 0, 0)
	if err != nil {
		return nil, err
	}
	levels := map[string][]*core.Record{}
	for _, threshold := range thresholds {
		careerID := threshold.GetString("career_identity_id")
		levels[careerID] = append(levels[careerID], threshold)
	}
	for _, total := range totals {
		identity := identities[total.CareerIdentityID]
		if identity == nil || total.Total <= 0 {
			continue
		}
		badge := centerBadge{
			CareerID:   identity.Id,
			CareerName: identity.GetString("name"),
			Category:   identity.GetString("category"),
			TotalXP:    total.Total,
		}
		for _, threshold := range levels[total.CareerIdentityID] {
			minimum := int64(threshold.GetInt("minimum_total_xp"))
			if total.Total >= minimum {
				badge.Level = threshold.GetInt("level_number")
				badge.LevelTitle = threshold.GetString("display_title")
				badge.LevelMinXP = minimum
				continue
			}
			badge.NextLevelXP = minimum
			badge.NextLevelTitle = threshold.GetString("display_title")
			break
		}
		if badge.Level == 0 {
			continue
		}
		badges = append(badges, badge)
	}
	sort.SliceStable(badges, func(i, j int) bool {
		if badges[i].Level != badges[j].Level {
			return badges[i].Level > badges[j].Level
		}
		if badges[i].TotalXP != badges[j].TotalXP {
			return badges[i].TotalXP > badges[j].TotalXP
		}
		return badges[i].CareerName < badges[j].CareerName
	})
	return badges, nil
}
