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

func TestAdminConsoleApplications(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := publicProjectTestUser(t, app, "console-owner@example.com", "负责人")
	admin := publicProjectTestUser(t, app, "console-admin@example.com", "管理员")
	admin.Set("role", "admin")
	publicProjectTestSave(t, app, admin)
	applicant := publicProjectTestUser(t, app, "console-applicant@example.com", "申请人")
	project := publicProjectTestProject(t, app, owner, "审批项目", "approved", "")

	request := core.NewRecord(publicProjectTestCollection(t, app, "project_join_requests"))
	request.Set("project_id", project.Id)
	request.Set("user_id", applicant.Id)
	request.Set("source", "application")
	request.Set("message", "希望参与结构设计")
	request.Set("status", "pending")
	publicProjectTestSave(t, app, request)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindAdminConsoleRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := admin.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	applicantToken, err := applicant.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if response := call("GET", "/api/hpahub/manage/members", applicantToken, ""); response.Code != http.StatusForbidden {
		t.Fatalf("member directory status = %d, want 403", response.Code)
	}
	response := call("GET", "/api/hpahub/manage/members", adminToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("admin directory status = %d: %s", response.Code, response.Body.String())
	}
	var members struct {
		Items []managedMember `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	foundApplicant := false
	for _, member := range members.Items {
		if member.Email == applicant.Email() {
			foundApplicant = true
		}
	}
	if !foundApplicant || len(members.Items) < 3 {
		t.Fatalf("members = %+v", members.Items)
	}
	response = call("GET", "/api/hpahub/manage/applications", adminToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("applications status = %d: %s", response.Code, response.Body.String())
	}
	var applications struct {
		Items []managedApplication `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &applications); err != nil || len(applications.Items) != 1 {
		t.Fatalf("applications = %+v, %v", applications.Items, err)
	}
	if response := call("PATCH", "/api/hpahub/manage/applications/"+request.Id, applicantToken, `{"status":"accepted"}`); response.Code != http.StatusForbidden {
		t.Fatalf("member approval status = %d, want 403", response.Code)
	}
	if response := call("PATCH", "/api/hpahub/manage/applications/"+request.Id, adminToken, `{"status":"accepted","note":"欢迎"}`); response.Code != http.StatusOK {
		t.Fatalf("approval status = %d: %s", response.Code, response.Body.String())
	}
	saved, err := app.FindRecordById("project_join_requests", request.Id)
	if err != nil || saved.GetString("status") != "accepted" || saved.GetString("handler_user_id") != admin.Id {
		t.Fatalf("saved request = %+v, %v", saved, err)
	}
	membership, err := app.FindFirstRecordByFilter("project_members", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": applicant.Id})
	if err != nil || membership.GetString("status") != "active" {
		t.Fatalf("membership = %+v, %v", membership, err)
	}
	if membership.GetDateTime("joined_at").Time().IsZero() || membership.GetDateTime("joined_at").Time().After(time.Now().Add(time.Minute)) {
		t.Fatalf("joined_at = %v", membership.GetDateTime("joined_at"))
	}
}
