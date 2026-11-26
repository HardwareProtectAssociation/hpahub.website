package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.Add(&core.FileField{Name: "avatar_thumb", MaxSelect: 1, MaxSize: 1 << 20, MimeTypes: []string{"image/webp"}, Protected: true})
		return app.Save(users)
	}, nil)
}
