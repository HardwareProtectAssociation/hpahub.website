package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestPublicProjectRoutes(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := publicProjectTestUser(t, app, "owner@example.com", "项目负责人")
	member := publicProjectTestUser(t, app, "member@example.com", "协作成员")
	approved := publicProjectTestProject(t, app, owner, "公开键盘项目", "approved", "")
	pending := publicProjectTestProject(t, app, owner, "待审核项目", "pending", "")
	deleted := publicProjectTestProject(t, app, owner, "已删除项目", "approved", time.Now().Format(time.RFC3339))

	stage := core.NewRecord(publicProjectTestCollection(t, app, "project_stages"))
	stage.Set("project_id", approved.Id)
	stage.Set("name", "方案")
	stage.Set("sort_order", 10)
	publicProjectTestSave(t, app, stage)
	approved.Set("current_stage_id", stage.Id)
	approved.Set("planned_start_date", "2027-01-01")
	approved.Set("schedule_note", "完成原型")
	approved.Set("collaboration_mode", "collaborative")
	approved.Set("join_mode", "approval")
	publicProjectTestSave(t, app, approved)

	membership := core.NewRecord(publicProjectTestCollection(t, app, "project_members"))
	membership.Set("project_id", approved.Id)
	membership.Set("user_id", member.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", time.Now())
	membership.Set("duties", "负责结构与打样")
	publicProjectTestSave(t, app, membership)

	follow := core.NewRecord(publicProjectTestCollection(t, app, "project_follows"))
	follow.Set("project_id", approved.Id)
	follow.Set("user_id", member.Id)
	follow.Set("is_active", true)
	follow.Set("receive_updates", true)
	follow.Set("followed_at", time.Now())
	publicProjectTestSave(t, app, follow)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		return response
	}

	list := request("/api/hpahub/projects")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []publicProject `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != approved.Id {
		t.Fatalf("unexpected public projects: %+v", listed.Items)
	}
	if listed.Items[0].CreatorName != "项目负责人" || listed.Items[0].ParticipantCount != 2 || len(listed.Items[0].Managers) != 1 || listed.Items[0].Managers[0].Name != "项目负责人" {
		t.Fatalf("unexpected project card: %+v", listed.Items[0])
	}
	if listed.Items[0].FollowCount != 1 {
		t.Fatalf("follow count = %d", listed.Items[0].FollowCount)
	}
	if listed.Items[0].OwnerName != "项目负责人" {
		t.Fatalf("owner name = %q", listed.Items[0].OwnerName)
	}

	detail := request("/api/hpahub/projects/" + approved.Id)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d: %s", detail.Code, detail.Body.String())
	}
	var project publicProjectDetail
	if err := json.Unmarshal(detail.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	if project.CurrentStage != "方案" || len(project.Stages) != 1 || len(project.Members) != 1 {
		t.Fatalf("unexpected detail: %+v", project)
	}
	if project.ScheduleNote != "完成原型" || project.PlannedStartDate == "" || project.CollaborationMode != "collaborative" || project.JoinMode != "approval" {
		t.Fatalf("missing project details: %+v", project)
	}
	if project.Members[0].Duties != "负责结构与打样" {
		t.Fatalf("unexpected member: %+v", project.Members[0])
	}
	if project.FollowCount != 1 {
		t.Fatalf("detail follow count = %d", project.FollowCount)
	}

	approved.Set("allow_others_management", true)
	publicProjectTestSave(t, app, approved)
	grant := core.NewRecord(publicProjectTestCollection(t, app, "project_permission_grants"))
	grant.Set("project_id", approved.Id)
	grant.Set("user_id", member.Id)
	grant.Set("permission_code", "manage")
	grant.Set("granted_by", owner.Id)
	grant.Set("granted_at", time.Now())
	publicProjectTestSave(t, app, grant)
	list = request("/api/hpahub/projects")
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items[0].Managers) != 2 || listed.Items[0].Managers[1].Name != "协作成员" || listed.Items[0].ParticipantCount != 2 {
		t.Fatalf("missing project manager: %+v", listed.Items[0])
	}
	member.Set("hidden_profile_sections", []string{"projects"})
	publicProjectTestSave(t, app, member)
	list = request("/api/hpahub/projects")
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !listed.Items[0].Managers[1].Redacted || strings.Contains(list.Body.String(), "协作成员") {
		t.Fatalf("hidden manager exposed: %s", list.Body.String())
	}
	grant.Set("revoked_at", time.Now())
	publicProjectTestSave(t, app, grant)
	list = request("/api/hpahub/projects")
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items[0].Managers) != 1 {
		t.Fatalf("revoked manager still listed: %+v", listed.Items[0])
	}

	for _, id := range []string{pending.Id, deleted.Id, "missing"} {
		response := request("/api/hpahub/projects/" + id)
		if response.Code != http.StatusNotFound {
			t.Fatalf("project %s status = %d, want 404", id, response.Code)
		}
	}
}

func TestProjectDifficultyAdmin(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := publicProjectTestUser(t, app, "difficulty-owner@example.com", "负责人")
	admin := publicProjectTestUser(t, app, "difficulty-admin@example.com", "管理员")
	admin.Set("role", "admin")
	publicProjectTestSave(t, app, admin)
	member := publicProjectTestUser(t, app, "difficulty-member@example.com", "成员")
	project := publicProjectTestProject(t, app, owner, "难度项目", "approved", "")

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	bindDifficultyVotingRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	adminToken, err := admin.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	memberToken, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	if response := request("GET", "/api/hpahub/manage/projects", memberToken, ""); response.Code != http.StatusForbidden {
		t.Fatalf("member list status = %d, want 403", response.Code)
	}
	if response := request("GET", "/api/hpahub/manage/projects", adminToken, ""); response.Code != http.StatusOK {
		t.Fatalf("admin list status = %d: %s", response.Code, response.Body.String())
	}
	// Difficulty is now set via voting, not direct PATCH
	path := "/api/hpahub/projects/" + project.Id + "/difficulty/vote"
	if response := request("POST", path, memberToken, `{"difficulty":"S","stance":"approve"}`); response.Code != http.StatusForbidden {
		t.Fatalf("member vote status = %d, want 403", response.Code)
	}
	if response := request("POST", path, adminToken, `{"difficulty":"Z","stance":"approve"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid vote status = %d, want 400", response.Code)
	}
	if response := request("POST", path, adminToken, `{"difficulty":"S","stance":"approve"}`); response.Code != http.StatusOK {
		t.Fatalf("admin vote status = %d: %s", response.Code, response.Body.String())
	}
	saved, err := app.FindRecordById("projects", project.Id)
	if err != nil {
		t.Fatal(err)
	}
	// Difficulty not updated yet (needs 3 approvals + owner confirm)
	if saved.GetString("difficulty") != "" {
		t.Fatalf("difficulty set before consensus: %q", saved.GetString("difficulty"))
	}
	list := request("GET", "/api/hpahub/projects", "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("public list status = %d", list.Code)
	}
	var listed struct {
		Items []publicProject `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("unexpected public projects: %+v", listed.Items)
	}
	// Difficulty should be empty since consensus not reached
	if listed.Items[0].Difficulty != "" {
		t.Fatalf("difficulty shown before consensus: %q", listed.Items[0].Difficulty)
	}
}

func TestCompletedProjectImprovement(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := publicProjectTestUser(t, app, "improve-owner@example.com", "负责人")
	member := publicProjectTestUser(t, app, "improve-member@example.com", "改进者")
	project := publicProjectTestProject(t, app, owner, "已完成项目", "approved", "")
	project.Set("execution_status", "completed")
	publicProjectTestSave(t, app, project)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := call("GET", "/api/hpahub/projects/"+project.Id, "")
	if response.Code != http.StatusOK {
		t.Fatalf("detail status = %d: %s", response.Code, response.Body.String())
	}
	var detail publicProjectDetail
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if !detail.CanRequestImprovement || detail.CanApply {
		t.Fatalf("unexpected flags: %+v", detail)
	}
	if response := call("POST", "/api/hpahub/projects/"+project.Id+"/apply", `{"message":"join"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("join completed project status = %d, want 400", response.Code)
	}
	if response := call("POST", "/api/hpahub/projects/"+project.Id+"/improvement", `{"message":"improve"}`); response.Code != http.StatusOK {
		t.Fatalf("improvement status = %d: %s", response.Code, response.Body.String())
	}
	request, err := app.FindFirstRecordByFilter("project_join_requests", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": member.Id})
	if err != nil || request.GetString("source") != "improvement" || request.GetString("status") != "pending" {
		t.Fatalf("request = %+v, %v", request, err)
	}
	response = call("GET", "/api/hpahub/projects/"+project.Id, "")
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil || !detail.ImprovementPending {
		t.Fatalf("pending detail = %+v, %v", detail, err)
	}
}

func publicProjectTestUser(t *testing.T, app core.App, email, name string) *core.Record {
	t.Helper()
	record := core.NewRecord(publicProjectTestCollection(t, app, "users"))
	record.SetEmail(email)
	record.SetRandomPassword()
	record.Set("role", "member")
	record.Set("display_name", name)
	record.Set("name", name)
	record.Set("username", strings.ReplaceAll(email, "@", "_"))
	publicProjectTestSave(t, app, record)
	return record
}

func publicProjectTestProject(t *testing.T, app core.App, owner *core.Record, title, reviewStatus, deletedAt string) *core.Record {
	t.Helper()
	project := core.NewRecord(publicProjectTestCollection(t, app, "projects"))
	project.Set("creator_user_id", owner.Id)
	project.Set("owner_user_id", owner.Id)
	project.Set("responsible_admin_id", owner.Id)
	project.Set("title", title)
	project.Set("summary", "项目摘要")
	project.Set("description", map[string]any{"text": "项目详细介绍"})
	project.Set("review_status", reviewStatus)
	project.Set("execution_status", "ongoing")
	project.Set("collaboration_mode", "collaborative")
	project.Set("join_mode", "open")
	project.Set("version", 1)
	project.Set("adventure_style", "project")
	project.Set("published_at", time.Now())
	if deletedAt != "" {
		project.Set("deleted_at", deletedAt)
	}
	publicProjectTestSave(t, app, project)
	return project
}

func publicProjectTestCollection(t *testing.T, app core.App, name string) *core.Collection {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	return collection
}

func publicProjectTestSave(t *testing.T, app core.App, record *core.Record) {
	t.Helper()
	if err := app.Save(record); err != nil {
		t.Fatalf("save %s: %v", record.Collection().Name, err)
	}
}

func TestOpenJoinRollsBackOnMembershipFailure(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := publicProjectTestUser(t, app, "join-owner@example.com", "owner")
	user := publicProjectTestUser(t, app, "join-user@example.com", "user")
	project := publicProjectTestProject(t, app, owner, "project", "approved", "")
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, _ := user.NewAuthToken()
	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/hpahub/projects/"+project.Id+"/apply", strings.NewReader(`{"message":"join"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	if _, err := app.DB().NewQuery(`CREATE TRIGGER fail_join BEFORE INSERT ON project_members BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Execute(); err != nil {
		t.Fatal(err)
	}
	if res := call(); res.Code != http.StatusInternalServerError {
		t.Fatalf("failed join: %d %s", res.Code, res.Body.String())
	}
	rows, err := app.FindRecordsByFilter("project_join_requests", "project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
	if err != nil || len(rows) != 0 {
		t.Fatalf("application persisted: %d %v", len(rows), err)
	}
	if _, err := app.DB().NewQuery(`DROP TRIGGER fail_join`).Execute(); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if call().Code == http.StatusOK {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent joins succeeded %d times", successes.Load())
	}
	rows, err = app.FindRecordsByFilter("project_join_requests", "project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
	if err != nil || len(rows) != 1 || rows[0].GetString("status") != "accepted" {
		t.Fatalf("concurrent applications: %v %v", rows, err)
	}
	if res := call(); res.Code != http.StatusBadRequest {
		t.Fatalf("duplicate join: %d %s", res.Code, res.Body.String())
	}
}
