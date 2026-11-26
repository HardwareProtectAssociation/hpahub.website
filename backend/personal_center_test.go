package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestPersonalCenterData(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	user := centerTestUser(t, app, "member@example.com", "member")
	other := centerTestUser(t, app, "other@example.com", "member")
	identity := centerTestIdentity(t, app)
	skill := centerTestSkill(t, app)

	relation := centerTestRecord(t, app, "user_identities")
	relation.Set("user_id", user.Id)
	relation.Set("identity_id", identity.Id)
	centerTestSave(t, app, relation)

	userSkill := centerTestRecord(t, app, "user_skills")
	userSkill.Set("user_id", user.Id)
	userSkill.Set("skill_id", skill.Id)
	userSkill.Set("proficiency", "熟练")
	userSkill.Set("ability_note", "做过两块键盘 PCB")
	centerTestSave(t, app, userSkill)

	preferences := centerTestRecord(t, app, "user_collaboration_preferences")
	preferences.Set("user_id", user.Id)
	preferences.Set("willingness", "open")
	preferences.Set("help_types", []string{"技术"})
	preferences.Set("expected_reward_types", []string{"money", "credit"})
	preferences.Set("expected_reward_note", "希望获得署名和材料补贴")
	preferences.Set("visibility", "members")
	centerTestSave(t, app, preferences)

	completed := centerTestProject(t, app, user, "已完成的键盘项目", "completed")
	completed.Set("difficulty", "B")
	centerTestSave(t, app, completed)
	joined := centerTestProject(t, app, other, "参与的固件项目", "ongoing")
	membership := centerTestRecord(t, app, "project_members")
	membership.Set("project_id", joined.Id)
	membership.Set("user_id", user.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", time.Now())
	membership.Set("duties", "负责外壳与结构")
	centerTestSave(t, app, membership)

	follow := centerTestRecord(t, app, "member_relations")
	follow.Set("actor_user_id", other.Id)
	follow.Set("target_user_id", user.Id)
	follow.Set("relation", "admire")
	centerTestSave(t, app, follow)

	// A confirmed task contribution and a closed ballot produce one career badge.
	task := centerTestRecord(t, app, "project_tasks")
	task.Set("project_id", completed.Id)
	task.Set("creator_user_id", user.Id)
	task.Set("title", "PCB 布线")
	task.Set("description", "完成键盘 PCB 布线")
	task.Set("status", "completed")
	centerTestSave(t, app, task)

	role := centerTestRecord(t, app, "project_participant_roles")
	role.Set("project_id", completed.Id)
	role.Set("user_id", user.Id)
	role.Set("identity_id", identity.Id)
	role.Set("status", "active")
	role.Set("assigned_by", user.Id)
	centerTestSave(t, app, role)

	contribution := centerTestRecord(t, app, "project_task_contributions")
	contribution.Set("task_id", task.Id)
	contribution.Set("user_id", user.Id)
	contribution.Set("career_identity_id", identity.Id)
	contribution.Set("contribution_note", "完成布线")
	contribution.Set("status", "confirmed")
	contribution.Set("participant_role_id", role.Id)
	centerTestSave(t, app, contribution)

	ballot := centerTestRecord(t, app, "task_xp_ballots")
	ballot.Set("task_id", task.Id)
	ballot.Set("round_number", 1)
	ballot.Set("opened_by", user.Id)
	ballot.Set("status", "closed")
	ballot.Set("rules_snapshot", map[string]any{"rule": "median"})
	ballot.Set("opens_at", time.Now())
	ballot.Set("closes_at", time.Now())
	ballot.Set("closed_at", time.Now())
	ballot.Set("eligible_count", 1)
	ballot.Set("valid_vote_count", 1)
	ballot.Set("final_xp", 120)
	centerTestSave(t, app, ballot)

	ledger := centerTestRecord(t, app, "career_xp_ledger")
	ledger.Set("user_id", user.Id)
	ledger.Set("career_identity_id", identity.Id)
	ledger.Set("contribution_id", contribution.Id)
	ledger.Set("ballot_id", ballot.Id)
	ledger.Set("entry_kind", "award")
	ledger.Set("xp_delta", 120)
	ledger.Set("business_key", "test:award:"+contribution.Id)
	ledger.Set("allocation_snapshot", map[string]any{"basisPoints": 10000})
	centerTestSave(t, app, ledger)

	data, err := personalCenterData(app, user)
	if err != nil {
		t.Fatal(err)
	}
	if !data.Profile.Member || data.Profile.Email != "member@example.com" || data.Profile.Name != "member@example.com" {
		t.Fatalf("unexpected profile: %+v", data.Profile)
	}
	if len(data.Identities) != 1 || data.Identities[0].Name != identity.GetString("name") {
		t.Fatalf("unexpected identities: %+v", data.Identities)
	}
	if len(data.Skills) != 1 || data.Skills[0].Name != skill.GetString("name") || data.Skills[0].Proficiency != "比较熟练" {
		t.Fatalf("unexpected skills: %+v", data.Skills)
	}
	if data.Preferences == nil || len(data.Preferences.ExpectedRewardTypes) != 2 {
		t.Fatalf("unexpected preferences: %+v", data.Preferences)
	}
	if len(data.Projects.Completed) != 1 || data.Projects.Completed[0].ID != completed.Id || data.Projects.Completed[0].Difficulty != "B" {
		t.Fatalf("unexpected completed projects: %+v", data.Projects.Completed)
	}
	if len(data.Projects.Participated) != 1 || data.Projects.Participated[0].ID != joined.Id {
		t.Fatalf("unexpected participating projects: %+v", data.Projects.Participated)
	}
	if data.Relations.AdmireCount != 1 {
		t.Fatalf("unexpected relations: %+v", data.Relations)
	}
	if len(data.Badges) != 1 {
		t.Fatalf("unexpected badges: %+v", data.Badges)
	}
	badge := data.Badges[0]
	if badge.Level != 2 || badge.MaxLevel != 100 || badge.TotalXP != 120 || badge.LevelMinXP != 100 || badge.NextLevelXP != 300 {
		t.Fatalf("unexpected badge values: %+v", badge)
	}
}

func TestPersonalCenterRoutePermissions(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	user := centerTestUser(t, app, "route@example.com", "member")
	superuser := centerTestSuperuser(t, app)
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/hpahub/me/center", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if response := request(""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	userToken, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	if response := request(userToken); response.Code != http.StatusOK {
		t.Fatalf("member status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	superuserToken, err := superuser.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	response := request(superuserToken)
	if response.Code != http.StatusOK {
		t.Fatalf("superuser status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if body := response.Body.String(); !strings.Contains(body, `"member":false`) {
		t.Fatalf("superuser center should not report a member profile: %s", body)
	}
}

func TestPersonalCenterSuperuserLinkedMember(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	superuser := centerTestSuperuser(t, app)
	member := centerTestUser(t, app, "octopus@example.com", "admin")
	data, err := personalCenterData(app, superuser)
	if err != nil {
		t.Fatal(err)
	}
	if !data.Profile.Member {
		t.Fatalf("superuser with matching member email should see the member profile: %+v", data.Profile)
	}
	if data.Profile.ID != member.Id {
		t.Fatalf("profile id = %q, want member %q", data.Profile.ID, member.Id)
	}
	if data.Profile.Role != "octopus" {
		t.Fatalf("superuser role = %q, want octopus", data.Profile.Role)
	}
}

func TestPersonalCenterProfileUpdate(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := centerTestUser(t, app, "profile@example.com", "member")
	superuser := centerTestSuperuser(t, app)
	linked := centerTestUser(t, app, "octopus@example.com", "admin")

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", "/api/hpahub/me/center/profile", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	if response := request("", `{"name":"x"}`); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", response.Code)
	}

	memberToken, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	response := request(memberToken, `{"name":"新昵称","bio":"新的个人简介","profilePublic":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("member update status = %d: %s", response.Code, response.Body.String())
	}
	var updated centerResponse
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Profile.Name != "新昵称" || updated.Profile.Bio != "新的个人简介" || !updated.Profile.ProfilePublic {
		t.Fatalf("unexpected updated profile: %+v", updated.Profile)
	}
	saved, err := app.FindRecordById("users", member.Id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.GetString("name") != "新昵称" || saved.GetString("display_name") != "新昵称" {
		t.Fatalf("name fields not updated: %q / %q", saved.GetString("name"), saved.GetString("display_name"))
	}
	if invalid := request(memberToken, `{"unknown":"x"}`); invalid.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", invalid.Code)
	}

	superuserToken, err := superuser.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	response = request(superuserToken, `{"name":"管理员昵称","bio":"管理员简介","profilePublic":false}`)
	if response.Code != http.StatusOK {
		t.Fatalf("superuser update status = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Profile.Member || updated.Profile.Name != "管理员昵称" {
		t.Fatalf("unexpected superuser-updated profile: %+v", updated.Profile)
	}
	savedLinked, err := app.FindRecordById("users", linked.Id)
	if err != nil {
		t.Fatal(err)
	}
	if savedLinked.GetString("display_name") != "管理员昵称" {
		t.Fatalf("linked member not updated: %q", savedLinked.GetString("display_name"))
	}
}

func TestPersonalCenterSkillsUpdate(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := centerTestUser(t, app, "skills@example.com", "member")
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/hpahub/me/center/skills", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	if response := request("PUT", "", `{"skills":[]}`); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", response.Code)
	}
	memberToken, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	body := `{"skills":[
		{"name":"我的自定义技能","proficiency":"我是这个领域的神！","abilityNote":"自己练出来的。"},
		{"name":"PCB 设计","proficiency":"比较熟练","abilityNote":"做过键盘 PCB。"}
	]}`
	response := request("PUT", memberToken, body)
	if response.Code != http.StatusOK {
		t.Fatalf("skills update status = %d: %s", response.Code, response.Body.String())
	}
	var updated centerResponse
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Skills) != 2 {
		t.Fatalf("skills = %+v", updated.Skills)
	}
	proficiencies := map[string]string{}
	for _, skill := range updated.Skills {
		proficiencies[skill.Name] = skill.Proficiency
	}
	if proficiencies["我的自定义技能"] != "我是这个领域的神！" || proficiencies["PCB 设计"] != "比较熟练" {
		t.Fatalf("unexpected proficiencies: %+v", proficiencies)
	}
	custom, err := app.FindFirstRecordByFilter("skills", `name = "我的自定义技能"`)
	if err != nil || custom.GetString("category") != "自定义" {
		t.Fatalf("custom skill not created: %v %+v", err, custom)
	}
	if rows, err := app.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": member.Id}); err != nil || len(rows) != 2 {
		t.Fatalf("user skills = %d, %v", len(rows), err)
	}

	if invalid := request("PUT", memberToken, `{"skills":[{"name":"x","proficiency":"随便","abilityNote":""}]}`); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid proficiency status = %d, want 400", invalid.Code)
	}
	if duplicate := request("PUT", memberToken, `{"skills":[{"name":"x","proficiency":"初学者","abilityNote":""},{"name":"x","proficiency":"略懂一二","abilityNote":""}]}`); duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate skill status = %d, want 400", duplicate.Code)
	}
	if missing := request("PUT", memberToken, `{}`); missing.Code != http.StatusBadRequest {
		t.Fatalf("missing skills status = %d, want 400", missing.Code)
	}

	response = request("PUT", memberToken, `{"skills":[]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("clear skills status = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Skills) != 0 {
		t.Fatalf("skills not cleared: %+v", updated.Skills)
	}
	if rows, err := app.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": member.Id}); err != nil || len(rows) != 0 {
		t.Fatalf("user skills not cleared: %d, %v", len(rows), err)
	}
}

func TestSkillLikes(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := centerTestUser(t, app, "skill-owner@example.com", "member")
	other := centerTestUser(t, app, "skill-other@example.com", "member")
	skill := centerTestSkill(t, app)
	userSkill := centerTestRecord(t, app, "user_skills")
	userSkill.Set("user_id", owner.Id)
	userSkill.Set("skill_id", skill.Id)
	userSkill.Set("proficiency", "初学者")
	centerTestSave(t, app, userSkill)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	ownerToken, err := owner.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := other.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	like := func(token string) (int, map[string]any) {
		req := httptest.NewRequest("POST", "/api/hpahub/skills/"+userSkill.Id+"/like", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		var body map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		return response.Code, body
	}
	if status, _ := like(ownerToken); status != http.StatusForbidden {
		t.Fatalf("self like status = %d, want 403", status)
	}
	status, body := like(otherToken)
	if status != http.StatusOK || body["liked"] != true || body["likeCount"].(float64) != 1 {
		t.Fatalf("endorsement = %d %+v", status, body)
	}
	data, err := personalCenterData(app, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Skills) != 1 || data.Skills[0].LikeCount != 1 || data.Skills[0].LikedByMe {
		t.Fatalf("unexpected owner skill likes: %+v", data.Skills)
	}
	status, body = like(otherToken)
	if status != http.StatusOK || body["liked"] != false || body["likeCount"].(float64) != 0 {
		t.Fatalf("unlike = %d %+v", status, body)
	}

	cases := map[int]int{0: 0, 10: 0, 11: 1, 29: 1, 30: 2, 59: 2, 60: 3, 99: 3, 100: 4}
	for count, tier := range cases {
		if got := centerLikeTier(count); got != tier {
			t.Fatalf("tier(%d) = %d, want %d", count, got, tier)
		}
	}
}

func TestPublicMemberSkills(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := centerTestUser(t, app, "owner-skills@example.com", "member")
	viewer := centerTestUser(t, app, "viewer-skills@example.com", "member")
	skill := centerTestSkill(t, app)
	userSkill := centerTestRecord(t, app, "user_skills")
	userSkill.Set("user_id", owner.Id)
	userSkill.Set("skill_id", skill.Id)
	userSkill.Set("proficiency", "比较熟练")
	userSkill.Set("ability_note", "公开技能说明")
	centerTestSave(t, app, userSkill)

	like := centerTestRecord(t, app, "user_skill_likes")
	like.Set("user_skill_id", userSkill.Id)
	like.Set("user_id", viewer.Id)
	centerTestSave(t, app, like)

	skills, err := publicMemberSkills(&core.RequestEvent{App: app, Auth: viewer}, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0]["likeCount"] != 1 || skills[0]["likedByMe"] != true || skills[0]["canEndorse"] != true {
		t.Fatalf("unexpected viewer skills: %+v", skills)
	}
	if skills[0]["proficiency"] != "比较熟练" {
		t.Fatalf("unexpected proficiency: %+v", skills[0])
	}
	skills, err = publicMemberSkills(&core.RequestEvent{App: app, Auth: owner}, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0]["likeCount"] != 1 || skills[0]["canEndorse"] != false {
		t.Fatalf("unexpected owner skills: %+v", skills)
	}
}

func TestPersonalCenterSkillCRUD(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := centerTestUser(t, app, "skill-crud@example.com", "member")
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	response := request("POST", "/api/hpahub/me/skills", `{"name":"页面作品技能","proficiency":"略懂一二","abilityNote":"说明","links":["https://example.com/a"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("create status = %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		UserSkillID string `json:"userSkillId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.UserSkillID == "" {
		t.Fatalf("create response: %s", response.Body.String())
	}
	path := "/api/hpahub/me/skills/" + created.UserSkillID
	response = request("PATCH", path, `{"name":"页面作品技能改","proficiency":"非常熟练","abilityNote":"新说明","links":["https://example.com/b"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", response.Code, response.Body.String())
	}
	data, err := personalCenterData(app, member)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Skills) != 1 || data.Skills[0].Name != "页面作品技能改" || len(data.Skills[0].Links) != 1 || data.Skills[0].Links[0] != "https://example.com/b" {
		t.Fatalf("unexpected skill: %+v", data.Skills)
	}
	if response := request("PATCH", path, `{"name":"x","proficiency":"非常熟练","links":["javascript:alert(1)"]}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid link status = %d, want 400", response.Code)
	}
	if response := request("DELETE", path, ""); response.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", response.Code, response.Body.String())
	}
	data, err = personalCenterData(app, member)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Skills) != 0 {
		t.Fatalf("skill not deleted: %+v", data.Skills)
	}
}

func TestProfileDetailsFields(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := centerTestUser(t, app, "profile-details@example.com", "member")
	identities, err := app.FindRecordsByFilter("collaboration_identities", "", "name", 4, 0)
	if err != nil || len(identities) < 4 {
		t.Fatalf("identities = %d, %v", len(identities), err)
	}
	ids := []string{identities[0].Id, identities[1].Id, identities[2].Id}

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPersonalCenterRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", "/api/hpahub/me/center/profile", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	payload, err := json.Marshal(map[string]any{
		"gender":        "male",
		"personality":   "偏内向，喜欢把复杂问题拆成可执行的小步骤。",
		"age":           30,
		"contacts":      []map[string]string{{"type": "email", "value": "member@example.com"}, {"type": "wechat", "value": "member-wechat"}},
		"activityAreas": []string{"线上", "上海"},
		"currentStatus": "busy",
		"identityIds":   ids,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := request(string(payload))
	if response.Code != http.StatusOK {
		t.Fatalf("details status = %d: %s", response.Code, response.Body.String())
	}
	var updated centerResponse
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Profile.Gender != "male" || updated.Profile.Age != 30 || len(updated.Profile.Contacts) != 2 || len(updated.Profile.ActivityAreas) != 2 || updated.Profile.CurrentStatus != "busy" {
		t.Fatalf("unexpected details: %+v", updated.Profile)
	}
	featured := 0
	for _, identity := range updated.Identities {
		if identity.Featured {
			featured++
		}
	}
	if featured != 3 || len(updated.IdentityOptions) < 4 {
		t.Fatalf("featured = %d, options = %d", featured, len(updated.IdentityOptions))
	}
	tooManyAreas, _ := json.Marshal(map[string]any{"activityAreas": []string{"线上", "上海", "北京"}})
	if response := request(string(tooManyAreas)); response.Code != http.StatusBadRequest {
		t.Fatalf("three areas status = %d, want 400", response.Code)
	}
	tooManyIdentities, _ := json.Marshal(map[string]any{"identityIds": []string{identities[0].Id, identities[1].Id, identities[2].Id, identities[3].Id}})
	if response := request(string(tooManyIdentities)); response.Code != http.StatusBadRequest {
		t.Fatalf("four identities status = %d, want 400", response.Code)
	}
}

func centerTestUser(t *testing.T, app core.App, email, role string) *core.Record {
	t.Helper()
	record := centerTestRecord(t, app, "users")
	record.SetEmail(email)
	record.SetRandomPassword()
	record.Set("role", role)
	record.Set("display_name", email)
	centerTestSave(t, app, record)
	return record
}

func centerTestSuperuser(t *testing.T, app core.App) *core.Record {
	t.Helper()
	record := centerTestRecord(t, app, core.CollectionNameSuperusers)
	record.SetEmail("octopus@example.com")
	record.SetRandomPassword()
	centerTestSave(t, app, record)
	return record
}

func centerTestIdentity(t *testing.T, app core.App) *core.Record {
	t.Helper()
	record, err := app.FindFirstRecordByFilter("collaboration_identities", `name = "电子工程师"`)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func centerTestSkill(t *testing.T, app core.App) *core.Record {
	t.Helper()
	record, err := app.FindFirstRecordByFilter("skills", `name = "PCB 设计"`)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func centerTestProject(t *testing.T, app core.App, owner *core.Record, title, status string) *core.Record {
	t.Helper()
	project := centerTestRecord(t, app, "projects")
	project.Set("creator_user_id", owner.Id)
	project.Set("owner_user_id", owner.Id)
	project.Set("title", title)
	project.Set("review_status", "approved")
	project.Set("execution_status", status)
	project.Set("collaboration_mode", "collaborative")
	project.Set("join_mode", "open")
	project.Set("version", 1)
	project.Set("adventure_style", "project")
	centerTestSave(t, app, project)
	return project
}

func centerTestRecord(t *testing.T, app core.App, collection string) *core.Record {
	t.Helper()
	model, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatal(err)
	}
	return core.NewRecord(model)
}

func centerTestSave(t *testing.T, app core.App, record *core.Record) {
	t.Helper()
	if err := app.Save(record); err != nil {
		t.Fatalf("save %s: %v", record.Collection().Name, err)
	}
}
