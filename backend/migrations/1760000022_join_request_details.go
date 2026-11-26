package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addJoinRequestDetails, nil)
}

// addJoinRequestDetails adds profession, time availability, and ability fields
// to project join requests.
func addJoinRequestDetails(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("project_join_requests")
	if err != nil {
		return err
	}
	collection.Fields.Add(
		&core.TextField{Name: "apply_profession", Max: 100},
		&core.TextField{Name: "apply_time", Max: 100},
		&core.TextField{Name: "apply_ability", Max: 200},
	)
	return app.Save(collection)
}
