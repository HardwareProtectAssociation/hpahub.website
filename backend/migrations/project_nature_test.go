package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"testing"
)

func TestProjectNatureMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projects.Fields.GetByName("project_nature").(*core.SelectField); !ok {
		t.Fatal("missing project nature field")
	}
	owner := coTestUser(t, app, "nature-upgrade@example.com")
	old := coTestProject(t, app, owner)
	old.Set("adventure_style", "boss")
	coTestSave(t, app, old)
	projects.Fields.RemoveByName("project_nature")
	if err := app.Save(projects); err != nil {
		t.Fatal(err)
	}
	if err := addProjectNature(app); err != nil {
		t.Fatal(err)
	}
	if err := addProjectNature(app); err != nil {
		t.Fatal(err)
	}
	upgraded, err := app.FindRecordById("projects", old.Id)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.GetString("project_nature") != "" || upgraded.GetString("adventure_style") != "boss" || upgraded.GetString("title") != old.GetString("title") {
		t.Fatal("existing project changed during migration")
	}
}
