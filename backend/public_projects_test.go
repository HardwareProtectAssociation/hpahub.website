package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	publicProjectTestSave(t, app, approved)

	membership := core.NewRecord(publicProjectTestCollection(t, app, "project_members"))
	membership.Set("project_id", approved.Id)
	membership.Set("user_id", member.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", time.Now())
	membership.Set("duties", "负责结构与打样")
	publicProjectTestSave(t, app, membership)

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
	if project.Members[0].Duties != "负责结构与打样" {
		t.Fatalf("unexpected member: %+v", project.Members[0])
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
	path := "/api/hpahub/manage/projects/" + project.Id
	if response := request("PATCH", path, memberToken, `{"difficulty":"S"}`); response.Code != http.StatusForbidden {
		t.Fatalf("member patch status = %d, want 403", response.Code)
	}
	if response := request("PATCH", path, adminToken, `{"difficulty":"Z"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid patch status = %d, want 400", response.Code)
	}
	if response := request("PATCH", path, adminToken, `{"difficulty":"S"}`); response.Code != http.StatusOK {
		t.Fatalf("admin patch status = %d: %s", response.Code, response.Body.String())
	}
	saved, err := app.FindRecordById("projects", project.Id)
	if err != nil || saved.GetString("difficulty") != "S" {
		t.Fatalf("saved difficulty = %q, %v", saved.GetString("difficulty"), err)
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
	if len(listed.Items) != 1 || listed.Items[0].Difficulty != "S" {
		t.Fatalf("unexpected public difficulty: %+v", listed.Items)
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
