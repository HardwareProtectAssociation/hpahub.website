package migrations

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRemoveLegacyAccounts(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := removeLegacyAccounts(app); err != nil {
		t.Fatalf("fresh database: %v", err)
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	old := core.NewRecord(users)
	old.SetRandomPassword()
	if err := app.Save(old); err != nil {
		t.Fatal(err)
	}
	keep := core.NewRecord(users)
	keep.SetEmail("keep@example.com")
	keep.SetRandomPassword()
	if err := app.Save(keep); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB().NewQuery("CREATE TABLE hpahub_legacy_accounts (user_id TEXT NOT NULL)").Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB().NewQuery("INSERT INTO hpahub_legacy_accounts VALUES ({:id}), ('missing')").Bind(dbx.Params{"id": old.Id}).Execute(); err != nil {
		t.Fatal(err)
	}
	if err := removeLegacyAccounts(app); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindRecordById("users", old.Id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old account remains: %v", err)
	}
	if _, err := app.FindRecordById("users", keep.Id); err != nil {
		t.Fatalf("unrelated account removed: %v", err)
	}
	var count int
	if err := app.DB().NewQuery("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='hpahub_legacy_accounts'").Row(&count); err != nil || count != 0 {
		t.Fatalf("legacy table remains: %d %v", count, err)
	}
	if err := removeLegacyAccounts(app); err != nil {
		t.Fatalf("repeat: %v", err)
	}
}
