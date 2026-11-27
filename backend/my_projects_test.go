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

func TestMyProjectsAndFollow(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := publicProjectTestUser(t, app, "mine@example.com", "我")
	other := publicProjectTestUser(t, app, "other@example.com", "别人")
	initiated := publicProjectTestProject(t, app, member, "我发起的", "approved", "")
	participated := publicProjectTestProject(t, app, other, "我参与的", "approved", "")
	followed := publicProjectTestProject(t, app, other, "我关注的", "approved", "")

	membership := core.NewRecord(publicProjectTestCollection(t, app, "project_members"))
	membership.Set("project_id", participated.Id)
	membership.Set("user_id", member.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", time.Now())
	publicProjectTestSave(t, app, membership)

	follow := core.NewRecord(publicProjectTestCollection(t, app, "project_follows"))
	follow.Set("project_id", followed.Id)
	follow.Set("user_id", member.Id)
	follow.Set("is_active", true)
	follow.Set("receive_updates", true)
	follow.Set("followed_at", time.Now())
	publicProjectTestSave(t, app, follow)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindMyProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := call("GET", "/api/hpahub/me/projects")
	if response.Code != http.StatusOK {
		t.Fatalf("my projects status = %d: %s", response.Code, response.Body.String())
	}
	var mine struct {
		Initiated    []myProject `json:"initiated"`
		Participated []myProject `json:"participated"`
		Followed     []myProject `json:"followed"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	if len(mine.Initiated) != 1 || mine.Initiated[0].ID != initiated.Id || len(mine.Participated) != 1 || mine.Participated[0].ID != participated.Id || len(mine.Followed) != 1 || mine.Followed[0].ID != followed.Id {
		t.Fatalf("unexpected my projects: %+v", mine)
	}
	response = call("POST", "/api/hpahub/projects/"+followed.Id+"/follow")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"following":false`) {
		t.Fatalf("unfollow = %d: %s", response.Code, response.Body.String())
	}
	response = call("POST", "/api/hpahub/projects/"+followed.Id+"/follow")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"following":true`) {
		t.Fatalf("follow = %d: %s", response.Code, response.Body.String())
	}
}

func TestCreateProjectDetails(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	member := publicProjectTestUser(t, app, "create@example.com", "创建者")
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindMyProjectRoutes(&core.ServeEvent{App: app, Router: router})
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/hpahub/me/projects", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := call(`{"title":" 新项目 ","executionStatus":"not_started","summary":"简介","description":"项目详情","proposedDifficulty":"A","adventureStyle":"boss","projectNature":"research","collaborationMode":"independent","joinMode":"approval","acceptsSponsorship":true,"allowOthersManagement":true,"confidentialityRequested":true,"confidentialityTerms":"测试协议","estimatedDurationDays":30,"plannedStartDate":"2027-01-01","scheduleNote":"计划"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("create = %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	project, err := app.FindRecordById("projects", result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if project.GetString("execution_status") != "not_started" || project.GetString("confidentiality_terms") != "测试协议" || project.GetString("project_nature") != "research" || project.GetString("title") != "新项目" || projectDescriptionText(project) != "项目详情" || project.GetString("collaboration_mode") != "independent" || project.GetString("join_mode") != "approval" || !project.GetBool("accepts_sponsorship") || !project.GetBool("allow_others_management") || !project.GetBool("confidentiality_requested") || project.GetInt("estimated_duration_days") != 30 || project.GetString("planned_start_date") == "" || project.GetString("schedule_note") != "计划" {
		t.Fatalf("project details not saved: %v", project)
	}
	other := publicProjectTestUser(t, app, "other-create@example.com", "其他人")
	otherToken, err := other.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/hpahub/projects/"+project.Id+"/apply", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+otherToken)
	req.Header.Set("Content-Type", "application/json")
	blocked := httptest.NewRecorder()
	mux.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("independent project application = %d: %s", blocked.Code, blocked.Body.String())
	}
	response = call(`{"title":"长期完成项目","executionStatus":"completed","longTerm":true,"projectNature":"learning","acceptsSponsorship":true,"allowOthersManagement":true,"confidentialityRequested":true,"confidentialityTerms":"测试协议"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("completed long-term project = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	longProject, err := app.FindRecordById("projects", result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if longProject.GetString("execution_status") != "completed" || !longProject.GetBool("is_long_term") || longProject.GetInt("estimated_duration_days") != 0 {
		t.Fatalf("completed long-term values not saved: %v", longProject)
	}
	detailRequest := httptest.NewRequest("GET", "/api/hpahub/projects/"+result.ID, nil)
	detailResponse := httptest.NewRecorder()
	mux.ServeHTTP(detailResponse, detailRequest)
	var detail struct {
		AcceptsSponsorship       bool   `json:"acceptsSponsorship"`
		AllowOthersManagement    bool   `json:"allowOthersManagement"`
		ConfidentialityRequested bool   `json:"confidentialityRequested"`
		LongTerm                 bool   `json:"longTerm"`
		ProjectNature            string `json:"projectNature"`
		Status                   string `json:"status"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detailResponse.Code != http.StatusOK || !detail.LongTerm || !detail.AcceptsSponsorship || !detail.AllowOthersManagement || !detail.ConfidentialityRequested || detail.ProjectNature != "learning" || detail.Status != "completed" {
		t.Fatalf("unexpected detail: %d %s", detailResponse.Code, detailResponse.Body.String())
	}
	for _, status := range []string{"paused", "long_term"} {
		response := call(`{"title":"状态测试","executionStatus":"` + status + `"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("create %s: %d %s", status, response.Code, response.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		saved, err := app.FindRecordById("projects", created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.GetString("execution_status") != status || saved.GetBool("is_long_term") != (status == "long_term") {
			t.Fatalf("status not saved: %s", status)
		}
	}
	for _, body := range []string{`{"title":"x","executionStatus":"invalid"}`, `{"title":"x","confidentialityRequested":true}`, `{"title":"x","projectNature":"invalid"}`, `{"title":"x","longTerm":true,"estimatedDurationDays":30}`, `{"title":"x","estimatedDurationDays":36501}`, `{"title":"x","plannedStartDate":"2027-02-30"}`, `{"title":"x","joinMode":"unknown"}`} {
		if response := call(body); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid input = %d: %s", response.Code, response.Body.String())
		}
	}
}
