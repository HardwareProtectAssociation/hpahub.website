package migrations

import (
	"errors"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectArrangements, nil) }
func addProjectArrangements(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	changed := false
	for _, name := range []string{"allow_others_management", "confidentiality_requested"} {
		if projects.Fields.GetByName(name) == nil {
			projects.Fields.Add(&core.BoolField{Name: name})
			changed = true
		}
	}
	if projects.Fields.GetByName("confidentiality_terms") == nil {
		projects.Fields.Add(&core.TextField{Name: "confidentiality_terms", Max: 10000})
		changed = true
	}
	status, ok := projects.Fields.GetByName("execution_status").(*core.SelectField)
	if !ok {
		return errors.New("project execution status field missing")
	}
	found := false
	for _, value := range status.Values {
		if value == "not_started" {
			found = true
		}
	}
	if !found {
		status.Values = append(status.Values, "not_started")
		changed = true
	}
	if changed {
		return app.Save(projects)
	}
	return nil
}
