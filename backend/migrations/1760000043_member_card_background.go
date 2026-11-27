package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addMemberCardBackgroundCapacity, nil) }

func addMemberCardBackgroundCapacity(app core.App) error {
	cards, err := app.FindCollectionByNameOrId("member_cards")
	if err != nil {
		return err
	}
	images, ok := cards.Fields.GetByName("images").(*core.FileField)
	if !ok {
		return nil
	}
	if images.MaxSelect < 5 {
		images.MaxSelect = 5
		return app.Save(cards)
	}
	return nil
}
