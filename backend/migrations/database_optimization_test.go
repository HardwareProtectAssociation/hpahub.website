package migrations

import (
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func optimizationTestSave(t *testing.T, app core.App, model core.Model) {
	t.Helper()
	if err := app.Save(model); err != nil {
		t.Fatal(err)
	}
}

func optimizationLegacySchema(t *testing.T, app core.App) {
	t.Helper()
	projects, _ := app.FindCollectionByNameOrId("projects")
	for _, name := range []string{"danger_level", "task_difficulty"} {
		projects.Fields.Add(&core.TextField{Id: projects.Fields.GetByName(name).GetId(), Name: name, Max: 32})
	}
	// Test fixture only: recreate the former text columns with unchanged data.
	if err := app.SaveNoValidate(projects); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hw_fields", "hw_tables"} {
		collection, _ := app.FindCollectionByNameOrId(name)
		collection.RemoveIndex("idx_" + name + "_type_key")
		optimizationTestSave(t, app, collection)
	}
	for _, index := range businessQueryIndexes {
		collection, _ := app.FindCollectionByNameOrId(index.collection)
		collection.RemoveIndex(index.name)
		optimizationTestSave(t, app, collection)
	}
}

func TestDatabaseOptimizationUpgrade(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := coTestUser(t, app, "db-upgrade@example.com")
	project := coTestProject(t, app, owner)
	project.Set("summary", "Existing project introduction")
	project.Set("danger_level", "high")
	project.Set("task_difficulty", "hard")
	coTestSave(t, app, project)

	// Reproduce the schema before these migrations without touching user data.
	optimizationLegacySchema(t, app)

	for range 2 {
		for _, migrate := range []func(core.App) error{addHardwareDefinitionKeys, constrainProjectClassifications, addBusinessQueryIndexes} {
			if err := migrate(app); err != nil {
				t.Fatal(err)
			}
		}
	}
	upgraded, err := app.FindRecordById("projects", project.Id)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.GetString("summary") != project.GetString("summary") || upgraded.GetString("danger_level") != "high" || upgraded.GetString("task_difficulty") != "hard" {
		t.Fatal("migration changed existing project data")
	}
	upgraded.Set("danger_level", "invalid")
	if err := app.Save(upgraded); err == nil {
		t.Fatal("unsupported classification was accepted")
	}
	for _, check := range []struct{ query, index string }{
		{`SELECT * FROM projects WHERE review_status='approved' AND deleted_at='' ORDER BY published_at DESC,updated DESC,id DESC LIMIT 21`, "idx_projects_public_feed"},
		{`SELECT * FROM hardware WHERE is_public=1 ORDER BY sort_order,name LIMIT 31`, "idx_hardware_public_order"},
		{`SELECT * FROM messages WHERE recipient_user_id='example' ORDER BY created DESC,id DESC LIMIT 50`, "idx_messages_recipient_created"},
	} {
		var plan []struct {
			Detail string `db:"detail"`
		}
		if err := app.DB().NewQuery("EXPLAIN QUERY PLAN " + check.query).All(&plan); err != nil {
			t.Fatal(err)
		}
		details := ""
		for _, step := range plan {
			details += step.Detail + "\n"
		}
		if !strings.Contains(details, check.index) || strings.Contains(details, "TEMP B-TREE") {
			t.Fatalf("unexpected query plan: %s", details)
		}
	}
}

func TestHardwareDefinitionKeys(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	var firstType, secondType *core.Record
	for _, key := range []string{"test-type-one", "test-type-two"} {
		collection, _ := app.FindCollectionByNameOrId("hw_types")
		typ := core.NewRecord(collection)
		typ.Set("key", key)
		typ.Set("label", key)
		coTestSave(t, app, typ)
		if firstType == nil {
			firstType = typ
		} else {
			secondType = typ
		}
	}
	for _, name := range []string{"hw_fields", "hw_tables"} {
		collection, _ := app.FindCollectionByNameOrId(name)
		makeDefinition := func(typeID string) *core.Record {
			r := core.NewRecord(collection)
			r.Set("type", typeID)
			r.Set("key", "resolution")
			r.Set("group", "Specs")
			r.Set("label", "Resolution")
			r.Set("title", "Resolution tests")
			return r
		}
		coTestSave(t, app, makeDefinition(firstType.Id))
		coTestSave(t, app, makeDefinition(secondType.Id))
		if err := app.Save(makeDefinition(firstType.Id)); err == nil {
			t.Fatalf("duplicate key accepted in %s", name)
		}
		results := make(chan error, 2)
		for range 2 {
			r := makeDefinition(firstType.Id)
			r.Set("key", "concurrent-key")
			go func() { results <- app.Save(r) }()
		}
		succeeded := 0
		for range 2 {
			if <-results == nil {
				succeeded++
			}
		}
		if succeeded != 1 {
			t.Fatalf("concurrent duplicate writes succeeded %d times in %s", succeeded, name)
		}
	}
}

func TestDatabaseOptimizationRestoredDataDirectory(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := coTestUser(t, app, "db-restore@example.com")
	project := coTestProject(t, app, owner)
	project.Set("summary", "Restored introduction")
	project.Set("danger_level", "medium")
	project.Set("task_difficulty", "normal")
	coTestSave(t, app, project)
	hardwareCollection, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		t.Fatal(err)
	}
	hardware := core.NewRecord(hardwareCollection)
	hardware.Set("kind", "general")
	hardware.Set("name", "Existing hardware report")
	hardware.Set("data", map[string]any{"model": "Test model", "measurements": []any{12.5, 0, false}})
	coTestSave(t, app, hardware)
	optimizationLegacySchema(t, app)
	for _, file := range []string{
		"1760000054_hardware_definition_keys.go",
		"1760000055_project_classification_constraints.go",
		"1760000056_business_query_indexes.go",
	} {
		// Only the disposable fixture is rewound to the pre-upgrade version.
		if _, err := app.DB().NewQuery("DELETE FROM _migrations WHERE file = {:file}").Bind(dbx.Params{"file": file}).Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.ResetBootstrapState(); err != nil {
		t.Fatal(err)
	}
	// Clone a closed data directory, as when restoring a backup, then let
	// PocketBase apply the registered migrations through its normal startup.
	restored, err := tests.NewTestApp(app.DataDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Cleanup()
	r, err := restored.FindRecordById("projects", project.Id)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetString("summary") != "Restored introduction" || r.GetString("danger_level") != "medium" || r.GetString("task_difficulty") != "normal" {
		t.Fatal("restoring the old data directory lost project values")
	}
	projects, _ := restored.FindCollectionByNameOrId("projects")
	if _, ok := projects.Fields.GetByName("danger_level").(*core.SelectField); !ok {
		t.Fatal("restored database did not apply the classification migration")
	}
	upgradedHardware, err := restored.FindRecordById("hardware", hardware.Id)
	if err != nil || upgradedHardware.GetString("data") != hardware.GetString("data") {
		t.Fatalf("restoring the old data directory lost hardware test data: %v", err)
	}
}

func TestDatabaseOptimizationRejectsConflictsWithoutDataLoss(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := coTestUser(t, app, "db-conflict@example.com")
	project := coTestProject(t, app, owner)
	projects, _ := app.FindCollectionByNameOrId("projects")
	for _, name := range []string{"danger_level", "task_difficulty"} {
		projects.Fields.Add(&core.TextField{Id: projects.Fields.GetByName(name).GetId(), Name: name})
	}
	if err := app.SaveNoValidate(projects); err != nil {
		t.Fatal(err)
	}
	project, _ = app.FindRecordById("projects", project.Id)
	project.Set("task_difficulty", "legacy-value")
	coTestSave(t, app, project)
	if err := constrainProjectClassifications(app); err == nil {
		t.Fatal("migration silently accepted unknown data")
	}
	projects, _ = app.FindCollectionByNameOrId("projects")
	if _, ok := projects.Fields.GetByName("danger_level").(*core.TextField); !ok {
		t.Fatal("failed migration partially converted schema")
	}
	unchanged, _ := app.FindRecordById("projects", project.Id)
	if unchanged.GetString("task_difficulty") != "legacy-value" {
		t.Fatal("failed migration discarded unknown data")
	}

	tables, _ := app.FindCollectionByNameOrId("hw_tables")
	tables.RemoveIndex("idx_hw_tables_type_key")
	optimizationTestSave(t, app, tables)
	fields, _ := app.FindCollectionByNameOrId("hw_fields")
	fields.RemoveIndex("idx_hw_fields_type_key")
	optimizationTestSave(t, app, fields)
	typ, err := app.FindFirstRecordByFilter("hw_types", "key = 'general'")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		r := core.NewRecord(tables)
		r.Set("type", typ.Id)
		r.Set("key", "duplicate")
		r.Set("title", "Preserve this definition")
		coTestSave(t, app, r)
	}
	if err := addHardwareDefinitionKeys(app); err == nil {
		t.Fatal("migration silently removed duplicate definitions")
	}
	fields, _ = app.FindCollectionByNameOrId("hw_fields")
	if containsIndex(fields.Indexes, "idx_hw_fields_type_key") {
		t.Fatal("failed migration partially added indexes")
	}
	count, err := app.CountRecords("hw_tables", dbx.HashExp{"type": typ.Id, "key": "duplicate"})
	if err != nil || count != 2 {
		t.Fatalf("conflicting records were changed: %d, %v", count, err)
	}
}
