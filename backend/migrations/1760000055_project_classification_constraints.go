package migrations

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(constrainProjectClassifications, nil) }

func constrainProjectClassifications(app core.App) error {
	return app.RunInTransaction(func(tx core.App) error {
		projects, err := tx.FindCollectionByNameOrId("projects")
		if err != nil {
			return err
		}
		for _, field := range []struct {
			name   string
			values []string
		}{
			{"danger_level", []string{"low", "medium", "high", "extreme"}},
			{"task_difficulty", []string{"easy", "normal", "hard", "extreme"}},
		} {
			params := dbx.Params{}
			placeholders := []string{"''"}
			for i, value := range field.values {
				key := fmt.Sprintf("value%d", i)
				params[key] = value
				placeholders = append(placeholders, "{:"+key+"}")
			}
			var invalid int
			if err := tx.DB().NewQuery("SELECT COUNT(*) FROM projects WHERE " + field.name + " NOT IN (" + strings.Join(placeholders, ",") + ")").Bind(params).Row(&invalid); err != nil {
				return err
			}
			if invalid > 0 {
				return fmt.Errorf("projects.%s has %d unsupported values; resolve them before upgrading (values were preserved)", field.name, invalid)
			}
			existing := projects.Fields.GetByName(field.name)
			if existing == nil {
				return fmt.Errorf("missing projects.%s", field.name)
			}
			if _, ok := existing.(*core.SelectField); ok {
				continue
			}
			if _, ok := existing.(*core.TextField); !ok {
				return fmt.Errorf("unexpected field type for projects.%s", field.name)
			}
			// PocketBase forbids changing field types in place. Copy through a
			// renamed column inside this transaction, then remove only the copy.
			legacyName := field.name + "_text_upgrade"
			if projects.Fields.GetByName(legacyName) != nil {
				return fmt.Errorf("temporary field projects.%s already exists", legacyName)
			}
			existing.SetName(legacyName)
			if err := tx.Save(projects); err != nil {
				return err
			}
			projects.Fields.Add(&core.SelectField{Name: field.name, Values: field.values, MaxSelect: 1})
			if err := tx.Save(projects); err != nil {
				return err
			}
			if _, err := tx.DB().NewQuery("UPDATE projects SET " + field.name + " = " + legacyName).Execute(); err != nil {
				return err
			}
			projects.Fields.RemoveByName(legacyName)
			if err := tx.Save(projects); err != nil {
				return err
			}
		}
		return nil
	})
}
