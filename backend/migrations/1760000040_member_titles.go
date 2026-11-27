package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addMemberTitles, nil) }

func addMemberTitles(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	offers, ok := projects.Fields.GetByName("provider_offers").(*core.SelectField)
	if !ok {
		return fmt.Errorf("provider offers field missing")
	}
	changed := false
	present := false
	for _, value := range offers.Values {
		if value == "title" {
			present = true
			break
		}
	}
	if !present {
		offers.Values = append(offers.Values, "title")
		changed = true
	}
	if offers.MaxSelect < len(offers.Values) {
		offers.MaxSelect = len(offers.Values)
		changed = true
	}
	if changed {
		if err := app.Save(projects); err != nil {
			return err
		}
	}

	if _, err := app.FindCollectionByNameOrId("user_titles"); err == nil {
		return nil
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	titles := core.NewBaseCollection("user_titles")
	titles.Fields.Add(
		&core.RelationField{Name: "user_id", CollectionId: users.Id, MaxSelect: 1, Required: true},
		&core.TextField{Name: "name", Required: true, Min: 1, Max: 10},
		&core.BoolField{Name: "is_official"},
		&core.RelationField{Name: "issued_by", CollectionId: users.Id, MaxSelect: 1},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.DateField{Name: "revoked_at"},
	)
	titles.Indexes = append(titles.Indexes, "CREATE UNIQUE INDEX idx_user_titles_user_name ON user_titles (user_id, name) WHERE revoked_at = ''")
	return app.Save(titles)
}
