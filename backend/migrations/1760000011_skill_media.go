package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addSkillMedia, nil)
}

// addSkillMedia adds portfolio images and reference links to user_skills.
func addSkillMedia(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("user_skills")
	if err != nil {
		return err
	}
	collection.Fields.Add(
		&core.FileField{
			Name:      "images",
			MaxSelect: 4,
			MaxSize:   5 << 20,
			MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
			Protected: true,
		},
		&core.JSONField{Name: "links", MaxSize: 1 << 20},
	)
	return app.Save(collection)
}
