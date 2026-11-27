package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectAgreementAndManagement(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := publicProjectTestUser(t, app, "owner-manage@example.com", "负责人")
	manager := publicProjectTestUser(t, app, "manager-manage@example.com", "管理人")
	applicant := publicProjectTestUser(t, app, "applicant-manage@example.com", "申请人")
	outsider := publicProjectTestUser(t, app, "outsider-manage@example.com", "其他人")
	project := publicProjectTestProject(t, app, owner, "项目", "approved", "")
	project.Set("allow_others_management", true)
	project.Set("confidentiality_requested", true)
	project.Set("confidentiality_terms", "保密条款")
	project.Set("join_mode", "approval")
	publicProjectTestSave(t, app, project)
	membership := core.NewRecord(publicProjectTestCollection(t, app, "project_members"))
	membership.Set("project_id", project.Id)
	membership.Set("user_id", manager.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", "2026-01-01 00:00:00.000Z")
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
	call := func(user *core.Record, method, path, body string) *httptest.ResponseRecorder {
		token, err := user.NewAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		mux.ServeHTTP(result, req)
		return result
	}
	path := "/api/hpahub/projects/" + project.Id
	if r := call(applicant, "POST", path+"/apply", `{"acceptedTerms":""}`); r.Code != http.StatusBadRequest {
		t.Fatalf("unsigned application: %d %s", r.Code, r.Body.String())
	}
	if r := call(applicant, "POST", path+"/apply", `{"acceptedTerms":"其他内容"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("wrong terms: %d %s", r.Code, r.Body.String())
	}
	if r := call(applicant, "POST", path+"/apply", `{"acceptedTerms":"保密条款"}`); r.Code != http.StatusOK {
		t.Fatalf("signed application: %d %s", r.Code, r.Body.String())
	}
	agreement, err := app.FindFirstRecordByFilter("project_agreements", `project_id = {:project} && participant_user_id = {:user}`, dbx.Params{"project": project.Id, "user": applicant.Id})
	if err != nil {
		t.Fatal(err)
	}
	if agreement.GetString("status") != "confirmed" || !strings.Contains(agreement.GetString("terms_snapshot"), "保密条款") {
		t.Fatalf("agreement not recorded: %v", agreement)
	}
	request, err := app.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": applicant.Id})
	if err != nil {
		t.Fatal(err)
	}
	grantPath := path + "/managers/" + manager.Id
	if r := call(outsider, "PATCH", grantPath, `{"manager":true}`); r.Code != http.StatusForbidden {
		t.Fatalf("outsider grant: %d", r.Code)
	}
	if r := call(owner, "PATCH", grantPath, `{"manager":true}`); r.Code != http.StatusOK {
		t.Fatalf("owner grant: %d %s", r.Code, r.Body.String())
	}
	if r := call(manager, "GET", path+"/management", ""); r.Code != http.StatusOK {
		t.Fatalf("manager access: %d %s", r.Code, r.Body.String())
	}
	if r := call(manager, "PATCH", path+"/applications/"+request.Id, `{"status":"accepted"}`); r.Code != http.StatusOK {
		t.Fatalf("manager approve: %d %s", r.Code, r.Body.String())
	}
	member, err := app.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user}`, dbx.Params{"project": project.Id, "user": applicant.Id})
	if err != nil || member.GetString("status") != "active" {
		t.Fatal("member not activated", err)
	}
	if r := call(owner, "PATCH", grantPath, `{"manager":false}`); r.Code != http.StatusOK {
		t.Fatalf("owner revoke: %d %s", r.Code, r.Body.String())
	}
	if r := call(manager, "GET", path+"/management", ""); r.Code != http.StatusForbidden {
		t.Fatalf("revoked access: %d", r.Code)
	}
	project.Set("execution_status", "completed")
	publicProjectTestSave(t, app, project)
	if r := call(outsider, "POST", path+"/improvement", `{"message":"建议"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("unsigned improvement: %d %s", r.Code, r.Body.String())
	}
	if r := call(outsider, "POST", path+"/improvement", `{"message":"建议","acceptedTerms":"保密条款"}`); r.Code != http.StatusOK {
		t.Fatalf("signed improvement: %d %s", r.Code, r.Body.String())
	}
}
