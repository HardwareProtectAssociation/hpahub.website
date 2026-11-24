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
		users.Fields.Add(&core.BoolField{Name: "profilePublic"})
		users.Fields.Add(&core.TextField{Name: "bio", Max: 600})
		avatar := users.Fields.GetByName("avatar").(*core.FileField)
		avatar.MaxSelect = 1
		avatar.MaxSize = 5 << 20
		avatar.MimeTypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}
		avatar.Protected = true
		if err := app.Save(users); err != nil {
			return err
		}

		for _, name := range []string{"member_cards", "showcase_projects"} {
			collection := core.NewBaseCollection(name)
			collection.Fields.Add(&core.TextField{Name: "title", Required: true, Max: 80})
			collection.Fields.Add(&core.TextField{Name: "text", Max: 280})
			collection.Fields.Add(&core.FileField{
				Name: "images", MaxSelect: 4, MaxSize: 5 << 20,
				MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"}, Protected: true,
			})
			collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
			collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
			if name == "member_cards" {
				collection.Fields.Add(&core.RelationField{Name: "owner", CollectionId: users.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
			} else {
				collection.Fields.Add(&core.BoolField{Name: "published"})
			}
			if err := app.Save(collection); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
