package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"testing"
)

func TestProjectArrangementsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		t.Fatal(err)
	}
	owner := coTestUser(t, app, "arrangements@example.com")
	old := coTestProject(t, app, owner)
	old.Set("accepts_sponsorship", true)
	coTestSave(t, app, old)
	for _, name := range []string{"allow_others_management", "confidentiality_requested"} {
		if _, ok := projects.Fields.GetByName(name).(*core.BoolField); !ok {
			t.Fatalf("missing %s", name)
		}
		projects.Fields.RemoveByName(name)
	}
	if err := app.Save(projects); err != nil {
		t.Fatal(err)
	}
	if err := addProjectArrangements(app); err != nil {
		t.Fatal(err)
	}
	if err := addProjectArrangements(app); err != nil {
		t.Fatal(err)
	}
	upgraded, err := app.FindRecordById("projects", old.Id)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.GetBool("allow_others_management") || upgraded.GetBool("confidentiality_requested") || !upgraded.GetBool("accepts_sponsorship") {
		t.Fatal("existing project changed")
	}
}
