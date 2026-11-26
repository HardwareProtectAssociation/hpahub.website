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

	hardware := centerTestRecord(t, app, "hardware")
	hardware.Set("kind", "general")
	hardware.Set("name", "热门显卡")
	centerTestSave(t, app, hardware)
	coldHardware := centerTestRecord(t, app, "hardware")
	coldHardware.Set("kind", "general")
	coldHardware.Set("name", "冷门显卡")
	centerTestSave(t, app, coldHardware)

	review := centerTestRecord(t, app, "hardware_reviews")
	review.Set("hardware_id", hardware.Id)
	review.Set("author_user_id", engineer.Id)
	review.Set("title", "好卡")
	review.Set("content", "实测很强")
	centerTestSave(t, app, review)
	reviewLike := centerTestRecord(t, app, "hardware_review_likes")
	reviewLike.Set("review_id", review.Id)
	reviewLike.Set("user_id", fan.Id)
	centerTestSave(t, app, reviewLike)

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
	hardwareItems, err := homeHotHardware(&core.RequestEvent{App: app})
	if err != nil {
		t.Fatal(err)
	}
	if len(hardwareItems) != 2 || hardwareItems[0].ID != hardware.Id || hardwareItems[0].LikeCount != 1 || hardwareItems[1].ID != coldHardware.Id || hardwareItems[1].LikeCount != 0 {
		t.Fatalf("unexpected hot hardware: %+v", hardwareItems)
	}
}
