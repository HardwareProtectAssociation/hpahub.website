package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		for _, name := range []string{"member_cards", "showcase_projects", "user_skills"} {
			collection, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			collection.Fields.GetByName("images").(*core.FileField).MaxSize = 2 << 20
			if err := app.Save(collection); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
