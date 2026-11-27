package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestProjectQueriesPaginationAndScopedFollows(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := publicProjectTestUser(t, app, "query-owner@example.com", "Owner")
	fan := publicProjectTestUser(t, app, "query-fan@example.com", "Fan")
	projects := make([]*core.Record, 0, 23)
	for i := range 23 {
		projects = append(projects, publicProjectTestProject(t, app, owner, fmt.Sprintf("Project %02d", i), "approved", ""))
	}
	deleted := publicProjectTestProject(t, app, owner, "Deleted", "approved", time.Now().Format(time.RFC3339))
	for i, project := range append(projects[:2:2], deleted) {
		follow := core.NewRecord(publicProjectTestCollection(t, app, "project_follows"))
		follow.Set("project_id", project.Id)
		follow.Set("user_id", fan.Id)
		follow.Set("is_active", i != 1)
		follow.Set("followed_at", time.Now())
		publicProjectTestSave(t, app, follow)
	}
	counts, err := projectFollowCounts(app, projects[:2])
	if err != nil || len(counts) != 1 || counts[projects[0].Id] != 1 {
		t.Fatalf("unexpected scoped counts: %v, %v", counts, err)
	}
	counts, err = projectFollowCounts(app, nil)
	if err != nil || len(counts) != 0 {
		t.Fatalf("empty project page returned unrelated counts: %v, %v", counts, err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindPublicProjectRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for page, offset := range []int{0, 20} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", fmt.Sprintf("/api/hpahub/projects?offset=%d", offset), nil))
		if response.Code != 200 {
			t.Fatalf("page failed: %d %s", response.Code, response.Body.String())
		}
		var body struct {
			Items   []publicProject `json:"items"`
			HasMore bool            `json:"hasMore"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := 20
		if page == 1 {
			want = 3
		}
		if len(body.Items) != want || body.HasMore != (page == 0) {
			t.Fatalf("unexpected page size/more: %d %v", len(body.Items), body.HasMore)
		}
		for _, item := range body.Items {
			if item.ID == deleted.Id || seen[item.ID] {
				t.Fatal("deleted or duplicate project in page")
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 23 {
		t.Fatalf("pagination omitted projects: %d", len(seen))
	}
}

func TestHotProjectsIncludesOlderPopularProject(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := publicProjectTestUser(t, app, "hot-owner@example.com", "Owner")
	fan := publicProjectTestUser(t, app, "hot-fan@example.com", "Fan")
	hot := publicProjectTestProject(t, app, owner, "Older popular project", "approved", "")
	follow := core.NewRecord(publicProjectTestCollection(t, app, "project_follows"))
	follow.Set("project_id", hot.Id)
	follow.Set("user_id", fan.Id)
	follow.Set("is_active", true)
	follow.Set("followed_at", time.Now())
	publicProjectTestSave(t, app, follow)
	for i := range 101 {
		publicProjectTestProject(t, app, owner, fmt.Sprintf("New %03d", i), "approved", "")
	}
	items, err := homeHotProjects(&core.RequestEvent{App: app})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 8 || items[0].ID != hot.Id || items[0].FollowCount != 1 {
		t.Fatalf("popular project omitted or ranking changed: %+v", items)
	}
}
