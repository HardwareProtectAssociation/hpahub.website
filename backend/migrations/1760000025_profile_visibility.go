package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProfileVisibility, nil) }

func addProfileVisibility(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.Fields.Add(&core.JSONField{Name: "hidden_profile_sections", MaxSize: 4096})
	return app.Save(users)
}
