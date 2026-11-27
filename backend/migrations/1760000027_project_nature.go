package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectNature, nil) }
func addProjectNature(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("project_nature") == nil {
		projects.Fields.Add(&core.SelectField{Name: "project_nature", Values: []string{"commercial", "learning", "showcase", "research"}, MaxSelect: 1})
		return app.Save(projects)
	}
	return nil
}
