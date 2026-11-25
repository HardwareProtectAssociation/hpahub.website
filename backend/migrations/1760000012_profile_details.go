package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addProfileDetails, nil)
}

// addProfileDetails adds the common profile fields and the featured flag for
// the professions a member chooses to display (max 3 enforced by the service).
func addProfileDetails(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.Fields.Add(
		&core.SelectField{Name: "gender", Values: []string{"male", "female", "nonbinary", "private"}, MaxSelect: 1},
		&core.TextField{Name: "personality", Max: 200},
		&core.NumberField{Name: "age", OnlyInt: true, Min: coFloat(1), Max: coFloat(120)},
		&core.JSONField{Name: "contacts", MaxSize: 1 << 20},
		&core.JSONField{Name: "activity_areas", MaxSize: 1 << 20},
	)
	if err := app.Save(users); err != nil {
		return err
	}
	relations, err := app.FindCollectionByNameOrId("user_identities")
	if err != nil {
		return err
	}
	relations.Fields.Add(&core.BoolField{Name: "featured"})
	return app.Save(relations)
}
