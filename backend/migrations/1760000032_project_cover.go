package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectCover, nil) }

func addProjectCover(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("cover") != nil {
		return nil
	}
	projects.Fields.Add(&core.FileField{
		Name:      "cover",
		MaxSelect: 1,
		MaxSize:   2 << 20,
		MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
	})
	return app.Save(projects)
}
