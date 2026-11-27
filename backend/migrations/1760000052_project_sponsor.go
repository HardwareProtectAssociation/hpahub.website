package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectSponsor, nil) }

func addProjectSponsor(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("sponsor") != nil {
		return nil
	}
	projects.Fields.Add(&core.TextField{Name: "sponsor", Max: 500})
	return app.Save(projects)
}
