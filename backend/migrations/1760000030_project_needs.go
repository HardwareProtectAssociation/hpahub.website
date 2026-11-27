package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectNeeds, nil) }

func addProjectNeeds(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("project_needs") != nil {
		return nil
	}
	projects.Fields.Add(&core.SelectField{
		Name:      "project_needs",
		Values:    []string{"funding", "technical", "equipment", "manpower", "promotion", "emotional"},
		MaxSelect: 6,
	})
	return app.Save(projects)
}
