package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(extendHardwareSchema, nil) }
func extendHardwareSchema(app core.App) error {
	fields, err := app.FindCollectionByNameOrId("hw_fields")
	if err != nil {
		return err
	}
	for _, field := range []core.Field{
		&core.SelectField{Name: "data_type", Values: []string{"text", "number", "boolean", "date", "select", "multiselect"}},
		&core.TextField{Name: "unit", Max: 40},
		&core.BoolField{Name: "required"},
		&core.JSONField{Name: "options", MaxSize: 16384},
		&core.TextField{Name: "help_text", Max: 300},
	} {
		if fields.Fields.GetByName(field.GetName()) == nil {
			fields.Fields.Add(field)
		}
	}
	if err := app.Save(fields); err != nil {
		return err
	}
	hardware, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		return err
	}
	if hardware.Fields.GetByName("is_public") == nil {
		hardware.Fields.Add(&core.BoolField{Name: "is_public"})
	}
	if hardware.Fields.GetByName("cover") == nil {
		hardware.Fields.Add(&core.FileField{Name: "cover", MaxSelect: 1, MaxSize: 5 << 20, MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"}})
	}
	return app.Save(hardware)
}
