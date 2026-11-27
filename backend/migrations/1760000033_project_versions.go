package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectVersions, nil) }

func addProjectVersions(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	versions := coNew("project_versions",
		coRelation("project_id", projects.Id, true),
		coNumber("version", true, true, coFloat(1), coFloat(1<<31-1)),
		coRelation("editor_id", users.Id, false),
		coJSON("snapshot", true),
	)
	coIndexes(versions, "CREATE UNIQUE INDEX idx_project_versions_project_version ON project_versions (project_id, version)")
	return app.Save(versions)
}
