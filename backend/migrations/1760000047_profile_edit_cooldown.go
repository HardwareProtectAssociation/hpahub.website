package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProfileEditCooldown, nil) }

func addProfileEditCooldown(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if users.Fields.GetByName("profile_last_edit_at") == nil {
		users.Fields.Add(&core.NumberField{Name: "profile_last_edit_at", OnlyInt: true})
		return app.Save(users)
	}
	return nil
}
