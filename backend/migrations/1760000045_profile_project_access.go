package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProfileProjectAccess, nil) }

func addProfileProjectAccess(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if users.Fields.GetByName("profile_project_access") == nil {
		users.Fields.Add(&core.JSONField{Name: "profile_project_access", MaxSize: 32768})
		return app.Save(users)
	}
	return nil
}
