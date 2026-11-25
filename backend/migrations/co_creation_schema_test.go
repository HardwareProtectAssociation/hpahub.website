package migrations

import (
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// coCollections lists the business tables from 项目共创平台_AI开发说明.md §8
// that this schema migration creates.
var coCollections = []string{
	"collaboration_identities",
	"user_identities",
	"skills",
	"user_skills",
	"user_collaboration_preferences",
	"user_showcases",
	"projects",
	"project_review_records",
	"project_stages",
	"project_members",
	"project_join_requests",
	"project_permission_grants",
	"project_permission_requests",
	"project_contents",
	"media_assets",
	"project_comments",
	"project_ratings",
	"project_follows",
	"project_rewards",
	"project_sponsorship_needs",
	"project_sponsorships",
	"project_agreements",
	"project_reward_fulfillments",
	"project_unions",
	"project_skill_needs",
	"project_union_members",
	"project_takeover_requests",
	"project_deletion_requests",
	"notifications",
	"audit_events",
	"reports",
	"project_tasks",
	"project_task_contributions",
	"task_xp_ballots",
	"task_xp_votes",
	"career_xp_ledger",
	"career_level_thresholds",
	"project_completion_ratings",
	"membership_invitation_codes",
	"project_participant_roles",
}

func TestCoCreationSchemaCollections(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	for _, name := range coCollections {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Errorf("missing collection %q: %v", name, err)
		}
	}

	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"display_name", "account_status", "registration_source", "role", "bio", "avatar"} {
		if users.Fields.GetByName(name) == nil {
			t.Errorf("users collection is missing field %q", name)
		}
	}

	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cover_asset_id", "current_stage_id", "creator_user_id", "owner_user_id", "responsible_admin_id"} {
		if projects.Fields.GetByName(name) == nil {
			t.Errorf("projects collection is missing field %q", name)
		}
	}

	indexes, err := app.TableIndexes("project_join_requests")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := indexes["idx_project_join_requests_pending"]; !ok {
		t.Errorf("missing pending-request unique index, got %v", indexes)
	}

	if got := coCount(t, app, "collaboration_identities"); got != len(coIdentitySeeds) {
		t.Errorf("identity seeds = %d, want %d", got, len(coIdentitySeeds))
	}
	if got := coCount(t, app, "skills"); got != len(coSkillSeeds) {
		t.Errorf("skill seeds = %d, want %d", got, len(coSkillSeeds))
	}
	if got := coCount(t, app, "career_level_thresholds"); got != len(coIdentitySeeds)*coCareerLevelCap {
		t.Errorf("career threshold seeds = %d, want %d", got, len(coIdentitySeeds)*coCareerLevelCap)
	}
}

func TestCareerLevelCurve(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	identity, err := app.FindFirstRecordByFilter("collaboration_identities", "category='main'")
	if err != nil {
		t.Fatal(err)
	}
	thresholds, err := app.FindRecordsByFilter(
		"career_level_thresholds",
		"career_identity_id = {:identity}",
		"level_number",
		0,
		0,
		dbx.Params{"identity": identity.Id},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(thresholds) != coCareerLevelCap {
		t.Fatalf("thresholds = %d, want %d", len(thresholds), coCareerLevelCap)
	}
	previous := -1
	for index, threshold := range thresholds {
		level := threshold.GetInt("level_number")
		xp := threshold.GetInt("minimum_total_xp")
		if level != index+1 {
			t.Fatalf("level at index %d = %d", index, level)
		}
		if xp != 50*level*(level-1) {
			t.Fatalf("level %d xp = %d, want %d", level, xp, 50*level*(level-1))
		}
		if xp <= previous {
			t.Fatalf("level %d xp is not increasing", level)
		}
		previous = xp
	}
	if got := thresholds[len(thresholds)-1].GetInt("minimum_total_xp"); got != 495000 {
		t.Fatalf("level %d xp = %d, want 495000", coCareerLevelCap, got)
	}
}

func TestCoCreationInvitationCodeConstraints(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	inviter := coTestUser(t, app, "inviter@example.com")
	invitee := coTestUser(t, app, "invitee@example.com")

	first := coTestRecord(t, app, "membership_invitation_codes")
	first.Set("inviter_user_id", inviter.Id)
	first.Set("code_hash", "hash-one")
	first.Set("status", "unused")
	coTestSave(t, app, first)

	// Unused codes keep invitee empty, so several codes may coexist.
	second := coTestRecord(t, app, "membership_invitation_codes")
	second.Set("inviter_user_id", inviter.Id)
	second.Set("code_hash", "hash-two")
	second.Set("status", "unused")
	coTestSave(t, app, second)

	// A used code binds exactly one invitee.
	first.Set("status", "used")
	first.Set("invitee_user_id", invitee.Id)
	first.Set("used_at", time.Now())
	coTestSave(t, app, first)

	second.Set("invitee_user_id", invitee.Id)
	coTestExpectSaveError(t, app, second)

	duplicate := coTestRecord(t, app, "membership_invitation_codes")
	duplicate.Set("inviter_user_id", inviter.Id)
	duplicate.Set("code_hash", "hash-one")
	duplicate.Set("status", "unused")
	coTestExpectSaveError(t, app, duplicate)
}

func TestCoCreationPendingRequestUniqueness(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := coTestUser(t, app, "owner@example.com")
	applicant := coTestUser(t, app, "applicant@example.com")
	project := coTestProject(t, app, owner)

	first := coTestRecord(t, app, "project_join_requests")
	first.Set("project_id", project.Id)
	first.Set("user_id", applicant.Id)
	first.Set("source", "application")
	first.Set("status", "pending")
	coTestSave(t, app, first)

	second := coTestRecord(t, app, "project_join_requests")
	second.Set("project_id", project.Id)
	second.Set("user_id", applicant.Id)
	second.Set("source", "application")
	second.Set("status", "pending")
	coTestExpectSaveError(t, app, second)

	// Only pending rows are unique; a decided request frees a new application.
	first.Set("status", "rejected")
	coTestSave(t, app, first)
	coTestSave(t, app, second)
}

func TestCoCreationCareerLedgerUniqueness(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := coTestUser(t, app, "ledger@example.com")
	project := coTestProject(t, app, owner)
	identity, err := app.FindFirstRecordByFilter("collaboration_identities", "category='main'")
	if err != nil {
		t.Fatal(err)
	}

	task := coTestRecord(t, app, "project_tasks")
	task.Set("project_id", project.Id)
	task.Set("creator_user_id", owner.Id)
	task.Set("title", "PCB 布线")
	task.Set("description", "完成键盘 PCB 布线并提交验收")
	task.Set("status", "open")
	coTestSave(t, app, task)

	role := coTestRecord(t, app, "project_participant_roles")
	role.Set("project_id", project.Id)
	role.Set("user_id", owner.Id)
	role.Set("identity_id", identity.Id)
	role.Set("status", "active")
	role.Set("assigned_by", owner.Id)
	coTestSave(t, app, role)

	contribution := coTestRecord(t, app, "project_task_contributions")
	contribution.Set("task_id", task.Id)
	contribution.Set("user_id", owner.Id)
	contribution.Set("career_identity_id", identity.Id)
	contribution.Set("contribution_note", "完成布线")
	contribution.Set("status", "confirmed")
	contribution.Set("participant_role_id", role.Id)
	coTestSave(t, app, contribution)

	ballot := coTestRecord(t, app, "task_xp_ballots")
	ballot.Set("task_id", task.Id)
	ballot.Set("round_number", 1)
	ballot.Set("opened_by", owner.Id)
	ballot.Set("status", "open")
	ballot.Set("rules_snapshot", map[string]any{"rule": "median"})
	ballot.Set("opens_at", time.Now())
	ballot.Set("closes_at", time.Now().Add(24*time.Hour))
	coTestSave(t, app, ballot)

	award := coTestRecord(t, app, "career_xp_ledger")
	award.Set("user_id", owner.Id)
	award.Set("career_identity_id", identity.Id)
	award.Set("contribution_id", contribution.Id)
	award.Set("ballot_id", ballot.Id)
	award.Set("entry_kind", "award")
	award.Set("xp_delta", 100)
	award.Set("business_key", "award:task:"+task.Id)
	award.Set("allocation_snapshot", map[string]any{"basis_points": 10000})
	coTestSave(t, app, award)

	duplicateAward := coTestRecord(t, app, "career_xp_ledger")
	duplicateAward.Set("user_id", owner.Id)
	duplicateAward.Set("career_identity_id", identity.Id)
	duplicateAward.Set("contribution_id", contribution.Id)
	duplicateAward.Set("ballot_id", ballot.Id)
	duplicateAward.Set("entry_kind", "award")
	duplicateAward.Set("xp_delta", 100)
	duplicateAward.Set("business_key", "award:task:"+task.Id+":retry")
	duplicateAward.Set("allocation_snapshot", map[string]any{"basis_points": 10000})
	coTestExpectSaveError(t, app, duplicateAward)

	reversal := coTestRecord(t, app, "career_xp_ledger")
	reversal.Set("user_id", owner.Id)
	reversal.Set("career_identity_id", identity.Id)
	reversal.Set("contribution_id", contribution.Id)
	reversal.Set("ballot_id", ballot.Id)
	reversal.Set("entry_kind", "reversal")
	reversal.Set("xp_delta", -100)
	reversal.Set("business_key", "reversal:task:"+task.Id)
	reversal.Set("allocation_snapshot", map[string]any{"reverses": award.Id})
	coTestSave(t, app, reversal)
}

func TestCoCreationRatingScoreBounds(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := coTestUser(t, app, "rating@example.com")
	project := coTestProject(t, app, owner)

	outOfRange := coTestRecord(t, app, "project_completion_ratings")
	outOfRange.Set("project_id", project.Id)
	outOfRange.Set("user_id", owner.Id)
	outOfRange.Set("score", 6)
	outOfRange.Set("status", "visible")
	coTestExpectSaveError(t, app, outOfRange)

	valid := coTestRecord(t, app, "project_completion_ratings")
	valid.Set("project_id", project.Id)
	valid.Set("user_id", owner.Id)
	valid.Set("score", 5)
	valid.Set("status", "visible")
	coTestSave(t, app, valid)
}

func coTestUser(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	record := coTestRecord(t, app, "users")
	record.SetEmail(email)
	record.SetRandomPassword()
	record.Set("role", "member")
	coTestSave(t, app, record)
	return record
}

func coTestProject(t *testing.T, app core.App, owner *core.Record) *core.Record {
	t.Helper()
	project := coTestRecord(t, app, "projects")
	project.Set("creator_user_id", owner.Id)
	project.Set("owner_user_id", owner.Id)
	project.Set("title", "DIY 分体键盘")
	project.Set("review_status", "approved")
	project.Set("execution_status", "ongoing")
	project.Set("collaboration_mode", "collaborative")
	project.Set("join_mode", "open")
	project.Set("version", 1)
	project.Set("adventure_style", "project")
	coTestSave(t, app, project)
	return project
}

func coTestRecord(t *testing.T, app core.App, collection string) *core.Record {
	t.Helper()
	model, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatal(err)
	}
	return core.NewRecord(model)
}

func coTestSave(t *testing.T, app core.App, record *core.Record) {
	t.Helper()
	if err := app.Save(record); err != nil {
		t.Fatalf("save %s: %v", record.Collection().Name, err)
	}
}

func coTestExpectSaveError(t *testing.T, app core.App, record *core.Record) {
	t.Helper()
	if err := app.Save(record); err == nil {
		t.Fatalf("save %s: expected an error, got nil", record.Collection().Name)
	}
}

func coCount(t *testing.T, app core.App, table string) int {
	t.Helper()
	var count int
	if err := app.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
