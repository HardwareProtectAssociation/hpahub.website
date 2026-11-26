package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestProtectLegacyHardware(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	collection, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.Set("kind", "general")
	record.Set("name", "preserve")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := protectLegacyHardware(app); err != nil {
		t.Fatalf("already upgraded: %v", err)
	}
	if _, err := app.DB().NewQuery(`DELETE FROM _migrations WHERE file = '1760000006_clear_hardware_data.go'`).Execute(); err != nil {
		t.Fatal(err)
	}
	if err := protectLegacyHardware(app); err == nil {
		t.Fatal("legacy data must block upgrade")
	}
	if _, err := app.FindRecordById("hardware", record.Id); err != nil {
		t.Fatalf("data was not preserved: %v", err)
	}
}
