package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addImprovementSource, nil)
}

// addImprovementSource allows completed projects to receive improvement
// applications through the existing join-request workflow.
func addImprovementSource(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("project_join_requests")
	if err != nil {
		return err
	}
	field, ok := collection.Fields.GetByName("source").(*core.SelectField)
	if !ok {
		return nil
	}
	for _, value := range field.Values {
		if value == "improvement" {
			return nil
		}
	}
	field.Values = append(field.Values, "improvement")
	return app.Save(collection)
}
