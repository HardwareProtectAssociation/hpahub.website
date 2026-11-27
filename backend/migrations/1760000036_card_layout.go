package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("member_cards")
		if err != nil {
			return err
		}
		if c.Fields.GetByName("layout") != nil {
			return nil
		}
		c.Fields.Add(&core.JSONField{Name: "layout", MaxSize: 16384})
		return app.Save(c)
	}, nil)
}
