package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addHardwareSubmitter, nil) }
func addHardwareSubmitter(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	hardware, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		return err
	}
	if hardware.Fields.GetByName("submitted_by") == nil {
		hardware.Fields.Add(&core.RelationField{Name: "submitted_by", CollectionId: users.Id, MaxSelect: 1})
	}
	return app.Save(hardware)
}
