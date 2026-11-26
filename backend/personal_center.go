package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type centerContact struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type centerProfile struct {
	ID            string          `json:"id"`
	Email         string          `json:"email"`
	Name          string          `json:"name"`
	Bio           string          `json:"bio"`
	AvatarURL     string          `json:"avatarUrl"`
	ProfilePublic bool            `json:"profilePublic"`
	Role          string          `json:"role"`
	Member        bool            `json:"member"`
	Gender        string          `json:"gender"`
	Personality   string          `json:"personality"`
	Age           int             `json:"age"`
	Contacts      []centerContact `json:"contacts"`
	ActivityAreas []string        `json:"activityAreas"`
	CurrentStatus string          `json:"currentStatus"`
}

type centerIdentity struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Featured bool   `json:"featured"`
}

type centerSkill struct {
	ID          string   `json:"id"`
	UserSkillID string   `json:"userSkillId"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Proficiency string   `json:"proficiency"`
	AbilityNote string   `json:"abilityNote"`
	Images      []string `json:"images"`
	Links       []string `json:"links"`
	LikeCount   int      `json:"likeCount"`
	LikeTier    int      `json:"likeTier"`
	LikedByMe   bool     `json:"likedByMe"`
	Featured    bool     `json:"featured"`
}

type centerSkillPayload struct {
	Name        string   `json:"name"`
	Proficiency string   `json:"proficiency"`
	AbilityNote string   `json:"abilityNote"`
	Links       []string `json:"links"`
	Featured    bool     `json:"featured"`
}

type centerSkillInput struct {
	Name        string `json:"name"`
	Proficiency string `json:"proficiency"`
	AbilityNote string `json:"abilityNote"`
}

type centerSkillsInput struct {
	Skills *[]centerSkillInput `json:"skills"`
}

// centerSkillProficiencies is the fixed self-reported proficiency scale.
var centerSkillProficiencies = []string{"初学者", "略懂一二", "比较熟练", "非常熟练", "你可以永远信任我", "我是这个领域的神！"}

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
	Difficulty     string `json:"difficulty"`
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
	MaxLevel       int    `json:"maxLevel"`
	TotalXP        int64  `json:"totalXp"`
	NextLevelXP    int64  `json:"nextLevelXp,omitempty"`
	NextLevelTitle string `json:"nextLevelTitle,omitempty"`
}

type centerProjects struct {
	Initiated    []centerProject `json:"initiated"`
	Participated []centerProject `json:"participated"`
	Completed    []centerProject `json:"completed"`
	Assisted     []centerProject `json:"assisted"`
	Followed     []centerProject `json:"followed"`
}

type centerResponse struct {
	Profile         centerProfile         `json:"profile"`
	Identities      []centerIdentity      `json:"identities"`
	IdentityOptions []centerIdentity      `json:"identityOptions"`
	Projects        centerProjects        `json:"projects"`
	Badges          []centerBadge         `json:"badges"`
	Skills          []centerSkill         `json:"skills"`
	Preferences     *centerPreferences    `json:"preferences"`
	Relations       memberRelationSummary `json:"relations"`
}

func centerSkillProficiency(value string) string {
	switch value {
	case "入门":
		return "初学者"
	case "熟练":
		return "比较熟练"
	case "精通":
		return "非常熟练"
	case "":
		return "初学者"
	}
	return value
}

func validCenterSkillProficiency(value string) bool {
	for _, option := range centerSkillProficiencies {
		if option == value {
			return true
		}
	}
	return false
}

var centerContactTypes = []string{"email", "wechat", "qq", "discord", "phone", "other"}
var centerGenders = []string{"male", "female", "nonbinary", "private"}

func validCenterGender(value string) bool {
	if value == "" {
		return true
	}
	for _, option := range centerGenders {
		if option == value {
			return true
		}
	}
	return false
}

func parseCenterContacts(record *core.Record) []centerContact {
	raw, err := json.Marshal(record.Get("contacts"))
	if err != nil {
		return []centerContact{}
	}
	var contacts []centerContact
	if err := json.Unmarshal(raw, &contacts); err != nil || contacts == nil {
		return []centerContact{}
	}
	return contacts
}

func normalizeCenterContacts(contacts []centerContact) ([]centerContact, bool) {
	if len(contacts) > 5 {
		return nil, false
	}
	out := make([]centerContact, 0, len(contacts))
	for _, contact := range contacts {
		contact.Type = strings.TrimSpace(contact.Type)
		contact.Value = strings.TrimSpace(contact.Value)
		validType := false
		for _, option := range centerContactTypes {
			if option == contact.Type {
				validType = true
				break
			}
		}
		if !validType || contact.Value == "" || utf8.RuneCountInString(contact.Value) > 200 {
			return nil, false
		}
		out = append(out, contact)
	}
	return out, true
}

func normalizeActivityAreas(values []string) ([]string, bool) {
	if len(values) > 2 {
		return nil, false
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] || utf8.RuneCountInString(value) > 40 {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, true
}

func parseCenterStrings(record *core.Record, field string) []string {
	raw, err := json.Marshal(record.Get(field))
	if err != nil {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return []string{}
	}
	return values
}

func inputField(input *contentInput, key string, out any) (bool, error) {
	raw, ok := input.values[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, out); err == nil {
		return true, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return true, err
	}
	return true, json.Unmarshal([]byte(text), out)
}

func updateFeaturedIdentities(app core.App, userID string, identityIDs []string) error {
	rows, err := app.FindRecordsByFilter("user_identities", "user_id = {:user}", "", 0, 0, dbx.Params{"user": userID})
	if err != nil {
		return err
	}
	existing := map[string]*core.Record{}
	for _, row := range rows {
		existing[row.GetString("identity_id")] = row
	}
	selected := map[string]bool{}
	for _, id := range identityIDs {
		identity, err := app.FindRecordById("collaboration_identities", id)
		if err != nil {
			return err
		}
		selected[identity.Id] = true
		row := existing[identity.Id]
		if row == nil {
			collection, err := app.FindCollectionByNameOrId("user_identities")
			if err != nil {
				return err
			}
			row = core.NewRecord(collection)
			row.Set("user_id", userID)
			row.Set("identity_id", identity.Id)
		}
		row.Set("featured", true)
		if err := app.Save(row); err != nil {
			return err
		}
	}
	for identityID, row := range existing {
		if selected[identityID] || !row.GetBool("featured") {
			continue
		}
		row.Set("featured", false)
		if err := app.Save(row); err != nil {
			return err
		}
	}
	return nil
}

func normalizeCenterSkillPayload(input centerSkillPayload) (centerSkillPayload, bool) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 80 || !validCenterSkillProficiency(input.Proficiency) || utf8.RuneCountInString(input.AbilityNote) > 500 {
		return input, false
	}
	if len(input.Links) > 5 {
		return input, false
	}
	links := make([]string, 0, len(input.Links))
	seen := map[string]bool{}
	for _, raw := range input.Links {
		link := strings.TrimSpace(raw)
		if link == "" || seen[link] {
			continue
		}
		parsed, err := url.Parse(link)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || utf8.RuneCountInString(link) > 500 {
			return input, false
		}
		seen[link] = true
		links = append(links, link)
	}
	input.Name = name
	input.Links = links
	return input, true
}

func findOrCreateCenterSkill(app core.App, name string) (*core.Record, error) {
	skill, err := app.FindFirstRecordByFilter("skills", "name = {:name}", dbx.Params{"name": name})
	if errors.Is(err, sql.ErrNoRows) {
		collection, err := app.FindCollectionByNameOrId("skills")
		if err != nil {
			return nil, err
		}
		skill = core.NewRecord(collection)
		skill.Set("name", name)
		skill.Set("category", "自定义")
		if err := app.Save(skill); err != nil {
			return nil, err
		}
		return skill, nil
	}
	if err != nil {
		return nil, err
	}
	return skill, nil
}

func centerOwnedSkill(app core.App, ownerID, id string) (*core.Record, error) {
	row, err := app.FindRecordById("user_skills", id)
	if err != nil || row.GetString("user_id") != ownerID {
		return nil, sql.ErrNoRows
	}
	return row, nil
}

func centerLikeTier(count int) int {
	switch {
	case count >= 5000:
		return 5
	case count >= 1000:
		return 4
	case count >= 100:
		return 3
	case count >= 50:
		return 2
	case count >= 10:
		return 1
	}
	return 0
}

type centerLikeStats struct {
	Total int `db:"total"`
	Mine  int `db:"mine"`
}

func centerSkillLikeStats(app core.App, userSkillID, viewerUserID, ownerUserID string) (int, bool, error) {
	var stats centerLikeStats
	err := app.DB().NewQuery(`
		SELECT COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN user_id = {:viewer} THEN 1 ELSE 0 END), 0) AS mine
		FROM user_skill_likes
		WHERE user_skill_id = {:skill} AND user_id != {:owner}
	`).Bind(dbx.Params{"skill": userSkillID, "viewer": viewerUserID, "owner": ownerUserID}).One(&stats)
	likedByMe := stats.Mine > 0 && viewerUserID != "" && viewerUserID != ownerUserID
	return stats.Total, likedByMe, err
}

// bindPersonalCenterRoutes exposes the signed-in account's own summary. It is
// not a public profile endpoint: it returns the caller's email and account
// details and requires a valid session. A superuser that also has a member
// record with the same email sees that member profile (used by platform
// operators that also participate in projects).
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
	e.Router.GET("/api/hpahub/me/center/avatar", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load avatar.", err)
		}
		if !member || owner.GetString("avatar") == "" {
			return e.NotFoundError("Not found.", nil)
		}
		return serveContentFile(e, owner, "avatar", owner.GetString("avatar"))
	})
	e.Router.PATCH("/api/hpahub/me/center/profile", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load profile.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		input, err := readContentInput(e, map[string]bool{
			"name": true, "bio": true, "profilePublic": true, "avatar": true, "removeAvatar": true,
			"gender": true, "personality": true, "age": true, "contacts": true, "activityAreas": true, "identityIds": true, "currentStatus": true,
		})
		if e.Request.MultipartForm != nil {
			defer e.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var name, bio string
		hasName, err := input.get("name", &name)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		hasBio, err := input.get("bio", &bio)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var profilePublic, removeAvatar bool
		hasPublic, err := input.get("profilePublic", &profilePublic)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		if _, err := input.get("removeAvatar", &removeAvatar); err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var gender, personality string
		hasGender, err := inputField(input, "gender", &gender)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		hasPersonality, err := inputField(input, "personality", &personality)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var age int
		hasAge, err := inputField(input, "age", &age)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var contacts []centerContact
		hasContacts, err := inputField(input, "contacts", &contacts)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var activityAreas []string
		hasAreas, err := inputField(input, "activityAreas", &activityAreas)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var identityIDs []string
		hasIdentities, err := inputField(input, "identityIds", &identityIDs)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		var currentStatus string
		hasCurrentStatus, err := inputField(input, "currentStatus", &currentStatus)
		if err != nil {
			return e.BadRequestError("Invalid profile.", err)
		}
		files := input.files["avatar"]
		if len(files) > 1 || (removeAvatar && len(files) > 0) {
			return e.BadRequestError("Invalid profile.", nil)
		}
		if hasName {
			name = strings.TrimSpace(name)
			if name == "" || utf8.RuneCountInString(name) > 80 {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("name", name)
			owner.Set("display_name", name)
		}
		if hasBio {
			if utf8.RuneCountInString(bio) > 600 {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("bio", bio)
		}
		if hasPublic {
			owner.Set("profilePublic", profilePublic)
		}
		if hasCurrentStatus {
			currentStatus = strings.TrimSpace(currentStatus)
			if currentStatus != "" && currentStatus != "busy" && currentStatus != "free" {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("current_status", currentStatus)
		}
		if hasGender {
			gender = strings.TrimSpace(gender)
			if !validCenterGender(gender) {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("gender", gender)
		}
		if hasPersonality {
			if utf8.RuneCountInString(personality) > 200 {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("personality", personality)
		}
		if hasAge {
			if age < 0 || age > 120 {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("age", age)
		}
		if hasContacts {
			normalized, ok := normalizeCenterContacts(contacts)
			if !ok {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("contacts", normalized)
		}
		if hasAreas {
			normalized, ok := normalizeActivityAreas(activityAreas)
			if !ok {
				return e.BadRequestError("Invalid profile.", nil)
			}
			owner.Set("activity_areas", normalized)
		}
		if hasIdentities && len(identityIDs) > 3 {
			return e.BadRequestError("Invalid profile.", nil)
		}
		if removeAvatar {
			owner.Set("avatar", "")
		} else if len(files) == 1 {
			owner.Set("avatar", files[0])
		}
		if owner.GetBool("profilePublic") && strings.TrimSpace(owner.GetString("name")) == "" {
			return e.BadRequestError("Invalid profile.", nil)
		}
		if err := e.App.Save(owner); err != nil {
			return e.BadRequestError("Unable to save profile.", err)
		}
		if hasIdentities {
			if err := updateFeaturedIdentities(e.App, owner.Id, identityIDs); err != nil {
				return e.BadRequestError("Unable to save profile.", err)
			}
		}
		data, err := personalCenterData(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to reload personal center.", err)
		}
		return e.JSON(200, data)
	})
	e.Router.PUT("/api/hpahub/me/center/skills", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input centerSkillsInput
		if err := e.BindBody(&input); err != nil || input.Skills == nil {
			return e.BadRequestError("Invalid skills.", err)
		}
		if len(*input.Skills) > 50 {
			return e.BadRequestError("Invalid skills.", nil)
		}
		items := make([]centerSkillInput, 0, len(*input.Skills))
		seen := map[string]bool{}
		for _, item := range *input.Skills {
			name := strings.TrimSpace(item.Name)
			if name == "" || utf8.RuneCountInString(name) > 80 || !validCenterSkillProficiency(item.Proficiency) || utf8.RuneCountInString(item.AbilityNote) > 500 {
				return e.BadRequestError("Invalid skills.", nil)
			}
			key := strings.ToLower(name)
			if seen[key] {
				return e.BadRequestError("Invalid skills.", nil)
			}
			seen[key] = true
			item.Name = name
			items = append(items, item)
		}
		if err := replacePersonalCenterSkills(e.App, owner.Id, items); err != nil {
			return e.InternalServerError("Unable to save skills.", err)
		}
		data, err := personalCenterData(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to reload personal center.", err)
		}
		return e.JSON(200, data)
	})
	e.Router.POST("/api/hpahub/me/skills", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input centerSkillPayload
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid skill.", err)
		}
		input, ok := normalizeCenterSkillPayload(input)
		if !ok {
			return e.BadRequestError("Invalid skill.", nil)
		}
		skill, err := findOrCreateCenterSkill(e.App, input.Name)
		if err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		if _, err := e.App.FindFirstRecordByFilter("user_skills", "user_id = {:user} && skill_id = {:skill}", dbx.Params{"user": owner.Id, "skill": skill.Id}); err == nil {
			return e.BadRequestError("Skill already exists.", nil)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return e.InternalServerError("Unable to save skill.", err)
		}
		collection, err := e.App.FindCollectionByNameOrId("user_skills")
		if err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		row := core.NewRecord(collection)
		row.Set("user_id", owner.Id)
		row.Set("skill_id", skill.Id)
		row.Set("proficiency", input.Proficiency)
		row.Set("ability_note", input.AbilityNote)
		row.Set("links", input.Links)
		row.Set("featured", input.Featured)
		if err := e.App.Save(row); err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "userSkillId": row.Id})
	})
	e.Router.PATCH("/api/hpahub/me/skills/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		row, err := centerOwnedSkill(e.App, owner.Id, e.Request.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("Not found.", nil)
		} else if err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		var input centerSkillPayload
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid skill.", err)
		}
		input, ok := normalizeCenterSkillPayload(input)
		if !ok {
			return e.BadRequestError("Invalid skill.", nil)
		}
		skill, err := findOrCreateCenterSkill(e.App, input.Name)
		if err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		if skill.Id != row.GetString("skill_id") {
			if _, err := e.App.FindFirstRecordByFilter("user_skills", "user_id = {:user} && skill_id = {:skill}", dbx.Params{"user": owner.Id, "skill": skill.Id}); err == nil {
				return e.BadRequestError("Skill already exists.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return e.InternalServerError("Unable to save skill.", err)
			}
			row.Set("skill_id", skill.Id)
		}
		row.Set("proficiency", input.Proficiency)
		row.Set("ability_note", input.AbilityNote)
		row.Set("links", input.Links)
		row.Set("featured", input.Featured)
		if err := e.App.Save(row); err != nil {
			return e.InternalServerError("Unable to save skill.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "userSkillId": row.Id})
	})
	e.Router.DELETE("/api/hpahub/me/skills/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		row, err := centerOwnedSkill(e.App, owner.Id, e.Request.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("Not found.", nil)
		} else if err != nil {
			return e.InternalServerError("Unable to delete skill.", err)
		}
		if err := e.App.Delete(row); err != nil {
			return e.InternalServerError("Unable to delete skill.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.POST("/api/hpahub/me/skills/{id}/images", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		row, err := centerOwnedSkill(e.App, owner.Id, e.Request.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("Not found.", nil)
		} else if err != nil {
			return e.InternalServerError("Unable to upload images.", err)
		}
		input, err := readContentInput(e, map[string]bool{"images": true})
		if e.Request.MultipartForm != nil {
			defer e.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			return e.BadRequestError("Invalid images.", err)
		}
		files := input.files["images"]
		if len(files) == 0 {
			return e.BadRequestError("Invalid images.", nil)
		}
		if len(row.GetStringSlice("images"))+len(files) > 4 {
			return e.BadRequestError("Too many images.", nil)
		}
		row.Set("images+", files)
		if err := e.App.Save(row); err != nil {
			return e.InternalServerError("Unable to upload images.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.DELETE("/api/hpahub/me/skills/{id}/images/{file}", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		row, err := centerOwnedSkill(e.App, owner.Id, e.Request.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("Not found.", nil)
		} else if err != nil {
			return e.InternalServerError("Unable to delete image.", err)
		}
		row.Set("images-", e.Request.PathValue("file"))
		if err := e.App.Save(row); err != nil {
			return e.InternalServerError("Unable to delete image.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.GET("/api/hpahub/skills/{id}/images/{file}", func(e *core.RequestEvent) error {
		noCache(e)
		row, err := e.App.FindRecordById("user_skills", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		return serveContentFile(e, row, "images", e.Request.PathValue("file"))
	})
	e.Router.POST("/api/hpahub/skills/{id}/like", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load skills.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		userSkill, err := e.App.FindRecordById("user_skills", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		if userSkill.GetString("user_id") == owner.Id {
			return e.ForbiddenError("You cannot endorse your own skill.", nil)
		}
		like, err := e.App.FindFirstRecordByFilter(
			"user_skill_likes",
			"user_skill_id = {:skill} && user_id = {:user}",
			dbx.Params{"skill": userSkill.Id, "user": owner.Id},
		)
		liked := false
		if errors.Is(err, sql.ErrNoRows) {
			collection, err := e.App.FindCollectionByNameOrId("user_skill_likes")
			if err != nil {
				return e.InternalServerError("Unable to like skill.", err)
			}
			record := core.NewRecord(collection)
			record.Set("user_skill_id", userSkill.Id)
			record.Set("user_id", owner.Id)
			if err := e.App.Save(record); err != nil {
				return e.InternalServerError("Unable to like skill.", err)
			}
			liked = true
		} else if err != nil {
			return e.InternalServerError("Unable to like skill.", err)
		} else if err := e.App.Delete(like); err != nil {
			return e.InternalServerError("Unable to like skill.", err)
		}
		count, _, err := centerSkillLikeStats(e.App, userSkill.Id, owner.Id, userSkill.GetString("user_id"))
		if err != nil {
			return e.InternalServerError("Unable to like skill.", err)
		}
		return e.JSON(200, map[string]any{"liked": liked, "likeCount": count, "likeTier": centerLikeTier(count)})
	})
}

// replacePersonalCenterSkills replaces the user's full skill list in one
// transaction. Custom names are added to the shared skills dictionary so they
// remain available for search and matching.
func replacePersonalCenterSkills(app core.App, userID string, items []centerSkillInput) error {
	return app.RunInTransaction(func(tx core.App) error {
		existing, err := tx.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": userID})
		if err != nil {
			return err
		}
		keep := map[string]bool{}
		for _, item := range items {
			skill, err := tx.FindFirstRecordByFilter("skills", "name = {:name}", dbx.Params{"name": item.Name})
			if errors.Is(err, sql.ErrNoRows) {
				collection, err := tx.FindCollectionByNameOrId("skills")
				if err != nil {
					return err
				}
				skill = core.NewRecord(collection)
				skill.Set("name", item.Name)
				skill.Set("category", "自定义")
				if err := tx.Save(skill); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			keep[skill.Id] = true
			userSkill, err := tx.FindFirstRecordByFilter(
				"user_skills",
				"user_id = {:user} && skill_id = {:skill}",
				dbx.Params{"user": userID, "skill": skill.Id},
			)
			if errors.Is(err, sql.ErrNoRows) {
				collection, err := tx.FindCollectionByNameOrId("user_skills")
				if err != nil {
					return err
				}
				userSkill = core.NewRecord(collection)
				userSkill.Set("user_id", userID)
				userSkill.Set("skill_id", skill.Id)
			} else if err != nil {
				return err
			}
			userSkill.Set("proficiency", item.Proficiency)
			userSkill.Set("ability_note", item.AbilityNote)
			if err := tx.Save(userSkill); err != nil {
				return err
			}
		}
		for _, row := range existing {
			if keep[row.GetString("skill_id")] {
				continue
			}
			if err := tx.Delete(row); err != nil {
				return err
			}
		}
		return nil
	})
}

// personalCenterOwner resolves the record whose member data should be shown.
// For a superuser this is the users record with the same email, when present.
func personalCenterOwner(app core.App, auth *core.Record) (*core.Record, bool, error) {
	if auth.Collection().Name == "users" {
		return auth, true, nil
	}
	if auth.Email() == "" {
		return auth, false, nil
	}
	member, err := app.FindAuthRecordByEmail("users", auth.Email())
	if errors.Is(err, sql.ErrNoRows) {
		return auth, false, nil
	}
	if err != nil {
		return auth, false, err
	}
	return member, true, nil
}

func personalCenterData(app core.App, auth *core.Record) (centerResponse, error) {
	owner, member, err := personalCenterOwner(app, auth)
	if err != nil {
		return centerResponse{}, err
	}
	response := centerResponse{
		Profile: centerProfile{
			ID:     owner.Id,
			Email:  owner.Email(),
			Role:   siteRole(auth),
			Member: member,
		},
		Identities:      []centerIdentity{},
		IdentityOptions: []centerIdentity{},
		Projects: centerProjects{
			Initiated:    []centerProject{},
			Participated: []centerProject{},
			Completed:    []centerProject{},
			Assisted:     []centerProject{},
			Followed:     []centerProject{},
		},
		Badges: []centerBadge{},
		Skills: []centerSkill{},
	}
	if !member {
		return response, nil
	}

	response.Profile.Name = owner.GetString("display_name")
	if response.Profile.Name == "" {
		response.Profile.Name = owner.GetString("name")
	}
	if response.Profile.Name == "" {
		response.Profile.Name = owner.Email()
	}
	response.Profile.Bio = owner.GetString("bio")
	response.Profile.ProfilePublic = owner.GetBool("profilePublic")
	response.Profile.Gender = owner.GetString("gender")
	response.Profile.Personality = owner.GetString("personality")
	response.Profile.Age = owner.GetInt("age")
	response.Profile.Contacts = parseCenterContacts(owner)
	response.Profile.ActivityAreas = parseCenterStrings(owner, "activity_areas")
	response.Profile.CurrentStatus = owner.GetString("current_status")
	if owner.GetString("avatar") != "" {
		response.Profile.AvatarURL = contentURL("me", "center", "avatar")
	}

	identities, err := centerIdentityRecords(app)
	if err != nil {
		return response, err
	}
	for _, identity := range identities {
		response.IdentityOptions = append(response.IdentityOptions, centerIdentity{
			ID:       identity.Id,
			Name:     identity.GetString("name"),
			Category: identity.GetString("category"),
		})
	}
	sort.SliceStable(response.IdentityOptions, func(i, j int) bool {
		if response.IdentityOptions[i].Category != response.IdentityOptions[j].Category {
			return response.IdentityOptions[i].Category < response.IdentityOptions[j].Category
		}
		return response.IdentityOptions[i].Name < response.IdentityOptions[j].Name
	})
	identityRows, err := app.FindRecordsByFilter("user_identities", "user_id = {:user}", "", 0, 0, dbx.Params{"user": owner.Id})
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
			Featured: row.GetBool("featured"),
		})
	}

	skills, err := centerSkillRecords(app)
	if err != nil {
		return response, err
	}
	skillRows, err := app.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": owner.Id})
	if err != nil {
		return response, err
	}
	for _, row := range skillRows {
		skill := skills[row.GetString("skill_id")]
		if skill == nil {
			continue
		}
		likeCount, likedByMe, err := centerSkillLikeStats(app, row.Id, owner.Id, row.GetString("user_id"))
		if err != nil {
			return response, err
		}
		imageNames := row.GetStringSlice("images")
		images := make([]string, 0, len(imageNames))
		for _, image := range imageNames {
			images = append(images, contentURL("skills", row.Id, "images", image))
		}
		response.Skills = append(response.Skills, centerSkill{
			ID:          skill.Id,
			UserSkillID: row.Id,
			Name:        skill.GetString("name"),
			Category:    skill.GetString("category"),
			Proficiency: centerSkillProficiency(row.GetString("proficiency")),
			AbilityNote: row.GetString("ability_note"),
			Images:      images,
			Links:       centerStrings(row.GetStringSlice("links")),
			LikeCount:   likeCount,
			LikeTier:    centerLikeTier(likeCount),
			LikedByMe:   likedByMe,
			Featured:    row.GetBool("featured"),
		})
	}

	preferences, err := app.FindFirstRecordByFilter(
		"user_collaboration_preferences",
		"user_id = {:user}",
		dbx.Params{"user": owner.Id},
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
			Difficulty:     project.GetString("difficulty"),
			AdventureStyle: project.GetString("adventure_style"),
			Relation:       relation,
			Duties:         duties,
			Updated:        project.GetString("updated"),
		}
		if relation == "owner" {
			response.Projects.Initiated = append(response.Projects.Initiated, item)
			if item.Status == "completed" {
				response.Projects.Completed = append(response.Projects.Completed, item)
			}
		} else {
			response.Projects.Participated = append(response.Projects.Participated, item)
			if item.Status == "completed" {
				response.Projects.Assisted = append(response.Projects.Assisted, item)
			}
		}
	}

	owned, err := app.FindRecordsByFilter("projects", "owner_user_id = {:user}", "-updated", 0, 0, dbx.Params{"user": owner.Id})
	if err != nil {
		return response, err
	}
	for _, project := range owned {
		addProject(project, "owner", "")
	}
	memberships, err := app.FindRecordsByFilter("project_members", `user_id = {:user} && status = "active"`, "", 0, 0, dbx.Params{"user": owner.Id})
	if err != nil {
		return response, err
	}
	for _, membership := range memberships {
		project, err := app.FindRecordById("projects", membership.GetString("project_id"))
		if err != nil {
			continue // removed project or stale membership
		}
		relation := "member"
		if project.GetString("owner_user_id") == owner.Id {
			relation = "owner"
		}
		addProject(project, relation, membership.GetString("duties"))
	}
	// Add followed projects (not already in initiated/participated)
	follows, err := app.FindRecordsByFilter("project_follows", `user_id = {:user} && is_active = true`, "", 0, 0, dbx.Params{"user": owner.Id})
	if err != nil {
		return response, err
	}
	for _, follow := range follows {
		project, err := app.FindRecordById("projects", follow.GetString("project_id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			continue
		}
		if seen[project.Id] {
			continue
		}
		seen[project.Id] = true
		item := centerProject{
			ID:             project.Id,
			Title:          project.GetString("title"),
			Summary:        project.GetString("summary"),
			Status:         project.GetString("execution_status"),
			ReviewStatus:   project.GetString("review_status"),
			Difficulty:     project.GetString("difficulty"),
			AdventureStyle: project.GetString("adventure_style"),
			Relation:       "followed",
			Updated:        project.GetString("updated"),
		}
		response.Projects.Followed = append(response.Projects.Followed, item)
	}
	sortCenterProjects(response.Projects.Initiated)
	sortCenterProjects(response.Projects.Participated)
	sortCenterProjects(response.Projects.Completed)
	sortCenterProjects(response.Projects.Assisted)
	sortCenterProjects(response.Projects.Followed)
	sort.SliceStable(response.Identities, func(i, j int) bool {
		if response.Identities[i].Category != response.Identities[j].Category {
			return response.Identities[i].Category < response.Identities[j].Category
		}
		return response.Identities[i].Name < response.Identities[j].Name
	})
	sort.SliceStable(response.Skills, func(i, j int) bool { return response.Skills[i].Name < response.Skills[j].Name })

	badges, err := centerBadges(app, owner.Id, identities)
	if err != nil {
		return response, err
	}
	response.Badges = badges
	relations, err := memberRelationSummaryData(app, owner.Id, "")
	if err != nil {
		return response, err
	}
	response.Relations = relations
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
			if level := threshold.GetInt("level_number"); level > badge.MaxLevel {
				badge.MaxLevel = level
			}
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
