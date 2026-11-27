package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectStatusReason, nil) }

func addProjectStatusReason(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("status_reason") != nil {
		return nil
	}
	projects.Fields.Add(coLongText("status_reason", false))
	return app.Save(projects)
}
