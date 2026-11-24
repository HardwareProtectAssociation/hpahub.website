package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		hardware := core.NewBaseCollection("hardware")
		hardware.Fields.Add(&core.TextField{Name: "kind", Required: true})
		hardware.Fields.Add(&core.TextField{Name: "name", Max: 1<<31 - 1})
		hardware.Fields.Add(&core.TextField{Name: "brand", Max: 1<<31 - 1})
		hardware.Fields.Add(&core.TextField{Name: "subtitle", Max: 1<<31 - 1})
		hardware.Fields.Add(&core.NumberField{Name: "sort_order"})
		hardware.Fields.Add(&core.JSONField{Name: "data", MaxSize: 1<<53 - 1})
		if err := app.Save(hardware); err != nil {
			return err
		}
		media := core.NewBaseCollection("hardware_media")
		media.Fields.Add(&core.RelationField{Name: "hardware", CollectionId: hardware.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
		media.Fields.Add(&core.JSONField{Name: "paths", MaxSize: 1<<53 - 1})
		media.Indexes = append(media.Indexes, "CREATE UNIQUE INDEX idx_hardware_media_hardware ON hardware_media (hardware)")
		if err := app.Save(media); err != nil {
			return err
		}
		options := core.NewBaseCollection("hardware_options")
		options.Fields.Add(&core.TextField{Name: "kind", Max: 1<<31 - 1})
		options.Fields.Add(&core.TextField{Name: "value", Max: 1<<31 - 1})
		options.Fields.Add(&core.NumberField{Name: "sort_order"})
		options.Indexes = append(options.Indexes, "CREATE UNIQUE INDEX idx_hardware_options_kind_value ON hardware_options (kind, value)")
		if err := app.Save(options); err != nil {
			return err
		}
		return nil
	}, nil)
}
