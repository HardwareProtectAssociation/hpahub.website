package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProfileBirthdayAndTitle, nil) }

func addProfileBirthdayAndTitle(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if users.Fields.GetByName("birthday") == nil {
		users.Fields.Add(&core.DateField{Name: "birthday"})
	}
	if users.Fields.GetByName("profile_title") == nil {
		users.Fields.Add(&core.TextField{Name: "profile_title", Max: 30})
	}
	return app.Save(users)
}
