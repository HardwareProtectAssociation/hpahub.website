package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectLongTerm, nil) }

func addProjectLongTerm(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("is_long_term") == nil {
		projects.Fields.Add(&core.BoolField{Name: "is_long_term"})
		return app.Save(projects)
	}
	return nil
}
