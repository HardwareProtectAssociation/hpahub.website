package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectRiskAndTaskDifficulty, nil) }

func addProjectRiskAndTaskDifficulty(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("danger_level") == nil {
		projects.Fields.Add(&core.TextField{Name: "danger_level", Max: 32})
	}
	if projects.Fields.GetByName("task_difficulty") == nil {
		projects.Fields.Add(&core.TextField{Name: "task_difficulty", Max: 32})
	}
	return app.Save(projects)
}
