package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"testing"
)

func TestProjectLongTermMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projects.Fields.GetByName("is_long_term").(*core.BoolField); !ok {
		t.Fatal("missing long-term field")
	}
	owner := coTestUser(t, app, "long-term-upgrade@example.com")
	old := coTestProject(t, app, owner)
	old.Set("estimated_duration_days", 30)
	coTestSave(t, app, old)
	projects.Fields.RemoveByName("is_long_term")
	if err := app.Save(projects); err != nil {
		t.Fatal(err)
	}
	if err := addProjectLongTerm(app); err != nil {
		t.Fatal(err)
	}
	if err := addProjectLongTerm(app); err != nil {
		t.Fatal(err)
	}
	upgraded, err := app.FindRecordById("projects", old.Id)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.GetBool("is_long_term") || upgraded.GetInt("estimated_duration_days") != 30 || upgraded.GetString("title") != old.GetString("title") {
		t.Fatal("existing project changed during migration")
	}
}
