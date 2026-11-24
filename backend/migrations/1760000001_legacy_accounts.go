package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		_, err := app.DB().NewQuery(`CREATE TABLE hpahub_legacy_accounts (
			legacy_id INTEGER PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL
		)`).Execute()
		return err
	}, func(app core.App) error {
		_, err := app.DB().NewQuery("DROP TABLE hpahub_legacy_accounts").Execute()
		return err
	})
}
