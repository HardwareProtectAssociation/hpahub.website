package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMemberRelations(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	target := centerTestUser(t, app, "relation-target@example.com", "member")
	target.Set("profilePublic", true)
	if err := app.Save(target); err != nil {
		t.Fatal(err)
	}
	actor := centerTestUser(t, app, "relation-actor@example.com", "member")
	actor.Set("profilePublic", true)
	if err := app.Save(actor); err != nil {
		t.Fatal(err)
	}
	token, err := actor.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindMemberRelationRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	call := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	path := "/api/hpahub/members/" + target.Id + "/relation"
	response := call(path, `{"relation":"admire"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("admire status = %d: %s", response.Code, response.Body.String())
	}
	var summary memberRelationSummary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil || summary.AdmireCount != 1 || summary.MyRelation != "admire" || !summary.CanRelate {
		t.Fatalf("summary = %+v, %v", summary, err)
	}
	response = call(path, `{"relation":"watch"}`)
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil || summary.AdmireCount != 0 || summary.WatchCount != 1 || summary.MyRelation != "watch" {
		t.Fatalf("switch summary = %+v, %v", summary, err)
	}
	response = call(path, `{"relation":"none"}`)
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil || summary.WatchCount != 0 || summary.MyRelation != "" {
		t.Fatalf("remove summary = %+v, %v", summary, err)
	}
	if response := call("/api/hpahub/members/"+actor.Id+"/relation", `{"relation":"admire"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("self relation status = %d, want 400", response.Code)
	}
}
