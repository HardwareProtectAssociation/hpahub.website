package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(replaceHardwareTemplate, nil)
}

// replaceHardwareTemplate removes the original GPU/CPU/device field templates
// and leaves one generic hardware form used by members for submissions.
func replaceHardwareTemplate(app core.App) error {
	types, err := app.FindCollectionByNameOrId("hw_types")
	if err != nil {
		return err
	}
	fields, err := app.FindCollectionByNameOrId("hw_fields")
	if err != nil {
		return err
	}
	tables, err := app.FindCollectionByNameOrId("hw_tables")
	if err != nil {
		return err
	}
	for _, collection := range []*core.Collection{fields, tables, types} {
		records, err := app.FindRecordsByFilter(collection, "", "", 0, 0)
		if err != nil {
			return err
		}
		for _, record := range records {
			if err := app.Delete(record); err != nil {
				return err
			}
		}
	}
	generic := core.NewRecord(types)
	generic.Set("key", "general")
	generic.Set("label", "硬件")
	generic.Set("name_field", "name")
	generic.Set("subtitle_fields", "brand,category")
	generic.Set("sort_order", 10)
	if err := app.Save(generic); err != nil {
		return err
	}
	fieldsList := []struct {
		key   string
		label string
	}{
		{"name", "名称"},
		{"brand", "品牌"},
		{"model", "型号"},
		{"category", "分类"},
		{"note", "备注"},
	}
	for index, field := range fieldsList {
		record := core.NewRecord(fields)
		record.Set("type", generic.Id)
		record.Set("group", "基本信息")
		record.Set("key", field.key)
		record.Set("label", field.label)
		record.Set("sort_order", index+1)
		if err := app.Save(record); err != nil {
			return err
		}
	}
	records, err := app.FindRecordsByFilter("hardware", "", "", 0, 0)
	if err != nil {
		return err
	}
	for _, record := range records {
		record.Set("kind", "general")
		if err := app.Save(record); err != nil {
			return err
		}
	}
	return nil
}
