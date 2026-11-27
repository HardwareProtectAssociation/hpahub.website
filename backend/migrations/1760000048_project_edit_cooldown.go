package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectEditCooldown, nil) }

func addProjectEditCooldown(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("project_last_edit_at") == nil {
		projects.Fields.Add(&core.NumberField{Name: "project_last_edit_at", OnlyInt: true})
		return app.Save(projects)
	}
	return nil
}
