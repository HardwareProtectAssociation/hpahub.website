package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addProjectDifficulty, nil)
}

// addProjectDifficulty adds the administrator-assigned D-S difficulty level.
func addProjectDifficulty(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	projects.Fields.Add(&core.SelectField{
		Name:      "difficulty",
		Values:    []string{"D", "C", "B", "A", "S"},
		MaxSelect: 1,
	})
	if err := app.Save(projects); err != nil {
		return err
	}
	rows, err := app.FindRecordsByFilter("projects", "", "", 0, 0)
	if err != nil {
		return err
	}
	for _, project := range rows {
		if project.GetString("difficulty") != "" {
			continue
		}
		project.Set("difficulty", "C")
		if err := app.Save(project); err != nil {
			return err
		}
	}
	return nil
}
