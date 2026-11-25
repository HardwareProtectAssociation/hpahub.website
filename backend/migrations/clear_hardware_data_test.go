package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestClearHardwareData(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := clearHardwareData(app); err != nil {
		t.Fatalf("fresh database: %v", err)
	}
	hardware, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		t.Fatal(err)
	}
	media, err := app.FindCollectionByNameOrId("hardware_media")
	if err != nil {
		t.Fatal(err)
	}
	options, err := app.FindCollectionByNameOrId("hardware_options")
	if err != nil {
		t.Fatal(err)
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}

	record := core.NewRecord(hardware)
	record.Set("kind", "gpu")
	record.Set("name", "GPU")
	record.Set("data", map[string]any{})
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	medium := core.NewRecord(media)
	medium.Set("hardware", record.Id)
	medium.Set("paths", []any{"https://example.com/image.png"})
	if err := app.Save(medium); err != nil {
		t.Fatal(err)
	}
	option := core.NewRecord(options)
	option.Set("kind", "game")
	option.Set("value", "Game")
	if err := app.Save(option); err != nil {
		t.Fatal(err)
	}
	keep := core.NewRecord(users)
	keep.SetEmail("keep@example.com")
	keep.SetRandomPassword()
	if err := app.Save(keep); err != nil {
		t.Fatal(err)
	}

	if err := clearHardwareData(app); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"hardware", "hardware_media", "hardware_options"} {
		var count int
		if err := app.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil || count != 0 {
			t.Fatalf("%s not cleared: %d rows, %v", table, count, err)
		}
	}
	if _, err := app.FindRecordById("users", keep.Id); err != nil {
		t.Fatalf("unrelated account removed: %v", err)
	}
	if err := clearHardwareData(app); err != nil {
		t.Fatalf("repeat: %v", err)
	}
}
