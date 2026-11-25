package migrations

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(removeLegacyAccounts, nil)
}

func removeLegacyAccounts(app core.App) error {
	var exists int
	if err := app.DB().NewQuery("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'hpahub_legacy_accounts'").Row(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	var ids []string
	if err := app.DB().NewQuery("SELECT user_id FROM hpahub_legacy_accounts").Column(&ids); err != nil {
		return err
	}
	for _, id := range ids {
		record, err := app.FindRecordById("users", id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if err := app.Delete(record); err != nil {
			return err
		}
	}
	_, err := app.DB().NewQuery("DROP TABLE hpahub_legacy_accounts").Execute()
	return err
}
