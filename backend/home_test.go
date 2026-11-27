package main

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestHomeFeed(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := centerTestUser(t, app, "home-owner@example.com", "member")
	fan := centerTestUser(t, app, "home-fan@example.com", "member")
	engineer := centerTestUser(t, app, "home-engineer@example.com", "member")
	engineer.Set("profilePublic", true)
	centerTestSave(t, app, engineer)

	hot := centerTestProject(t, app, owner, "热门项目", "ongoing")
	cold := centerTestProject(t, app, owner, "冷门项目", "ongoing")

	follow := centerTestRecord(t, app, "project_follows")
	follow.Set("project_id", hot.Id)
	follow.Set("user_id", fan.Id)
	follow.Set("is_active", true)
	follow.Set("followed_at", time.Now())
	centerTestSave(t, app, follow)

	admire := centerTestRecord(t, app, "member_relations")
	admire.Set("actor_user_id", fan.Id)
	admire.Set("target_user_id", engineer.Id)
	admire.Set("relation", "admire")
	centerTestSave(t, app, admire)

	projects, err := homeHotProjects(&core.RequestEvent{App: app})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != hot.Id || projects[0].FollowCount != 1 || projects[1].ID != cold.Id || projects[1].FollowCount != 0 {
		t.Fatalf("unexpected hot projects: %+v", projects)
	}
	engineers, err := homeHotEngineers(&core.RequestEvent{App: app})
	if err != nil {
		t.Fatal(err)
	}
	if len(engineers) != 1 || engineers[0].ID != engineer.Id || engineers[0].AdmireCount != 1 {
		t.Fatalf("unexpected hot engineers: %+v", engineers)
	}
}
