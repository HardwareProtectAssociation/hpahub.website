package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		collection := core.NewBaseCollection("warehouse_items")
		collection.Fields.Add(&core.TextField{Name: "path", Required: true, Max: 4096})
		collection.Fields.Add(&core.BoolField{Name: "directory"})
		collection.Fields.Add(&core.TextField{Name: "mode", Required: true, Max: 32})
		collection.Fields.Add(&core.TextField{Name: "modified", Required: true, Max: 32})
		collection.Fields.Add(&core.TextField{Name: "created", Max: 32})
		collection.Fields.Add(&core.TextField{Name: "digest", Max: 64})
		collection.Fields.Add(&core.FileField{Name: "content", MaxSelect: 1, MaxSize: 1<<53 - 1, Protected: true})
		collection.Indexes = append(collection.Indexes, "CREATE UNIQUE INDEX idx_warehouse_items_path ON warehouse_items (path)")
		if err := app.Save(collection); err != nil {
			return err
		}
		imports := core.NewBaseCollection("warehouse_imports")
		imports.Fields.Add(&core.TextField{Name: "version", Required: true, Max: 16})
		imports.Fields.Add(&core.TextField{Name: "manifest", Required: true, Max: 64})
		imports.Indexes = append(imports.Indexes, "CREATE UNIQUE INDEX idx_warehouse_imports_version ON warehouse_imports (version)")
		return app.Save(imports)
	}, nil)
}
