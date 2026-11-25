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
