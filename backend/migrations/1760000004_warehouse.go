package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		projects := core.NewBaseCollection("warehouse_projects")
		projects.Fields.Add(&core.TextField{Name: "name", Required: true, Max: 255})
		projects.Fields.Add(&core.JSONField{Name: "metadata"})
		projects.Indexes = append(projects.Indexes, "CREATE UNIQUE INDEX idx_warehouse_projects_name ON warehouse_projects (name)")
		if err := app.Save(projects); err != nil {
			return err
		}
		files := core.NewBaseCollection("warehouse_files")
		files.Fields.Add(&core.RelationField{Name: "project", CollectionId: projects.Id, Required: true, MaxSelect: 1, CascadeDelete: true})
		files.Fields.Add(&core.TextField{Name: "category", Required: true, Max: 255})
		files.Fields.Add(&core.TextField{Name: "name", Required: true, Max: 255})
		for _, name := range []string{"content", "thumbnail", "web", "preview"} {
			files.Fields.Add(&core.FileField{Name: name, MaxSelect: 1, MaxSize: 1<<53 - 1, Protected: true, Required: name == "content"})
		}
		files.Fields.Add(&core.JSONField{Name: "metadata"})
		files.Indexes = append(files.Indexes, "CREATE UNIQUE INDEX idx_warehouse_files_identity ON warehouse_files (project, category, name)")
		if err := app.Save(files); err != nil {
			return err
		}
		settings := core.NewBaseCollection("warehouse_settings")
		settings.Fields.Add(&core.TextField{Name: "key", Required: true, Max: 255})
		settings.Fields.Add(&core.JSONField{Name: "value"})
		settings.Indexes = append(settings.Indexes, "CREATE UNIQUE INDEX idx_warehouse_settings_key ON warehouse_settings (key)")
		return app.Save(settings)
	}, nil)
}
