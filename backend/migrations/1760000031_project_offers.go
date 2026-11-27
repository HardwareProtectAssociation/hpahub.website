package migrations

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectOffers, nil) }

func addProjectOffers(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	needs, ok := projects.Fields.GetByName("project_needs").(*core.SelectField)
	if !ok {
		return errors.New("project needs field missing")
	}
	changed := false
	hasVenue := false
	for _, value := range needs.Values {
		if value == "venue" {
			hasVenue = true
		}
	}
	if !hasVenue {
		needs.Values = append(needs.Values, "venue")
		changed = true
	}
	if projects.Fields.GetByName("provider_offers") == nil {
		projects.Fields.Add(&core.SelectField{
			Name:      "provider_offers",
			Values:    []string{"technical", "funding", "equipment", "venue", "manpower", "promotion"},
			MaxSelect: 6,
		})
		changed = true
	}
	if changed {
		return app.Save(projects)
	}
	return nil
}
