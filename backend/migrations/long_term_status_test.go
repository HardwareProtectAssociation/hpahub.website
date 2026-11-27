package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"testing"
)

func TestLongTermStatusMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		t.Fatal(err)
	}
	status := projects.Fields.GetByName("execution_status").(*core.SelectField)
	found := false
	for _, value := range status.Values {
		if value == "long_term" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing long-term status")
	}
	owner := coTestUser(t, app, "status-upgrade@example.com")
	old := coTestProject(t, app, owner)
	values := []string{}
	for _, value := range status.Values {
		if value != "long_term" {
			values = append(values, value)
		}
	}
	status.Values = values
	if err := app.Save(projects); err != nil {
		t.Fatal(err)
	}
	if err := addLongTermStatus(app); err != nil {
		t.Fatal(err)
	}
	upgraded, err := app.FindRecordById("projects", old.Id)
	if err != nil || upgraded.GetString("title") != old.GetString("title") || upgraded.GetString("execution_status") != "ongoing" {
		t.Fatal("existing project changed during upgrade")
	}
	if err := addLongTermStatus(app); err != nil {
		t.Fatal(err)
	}
}
