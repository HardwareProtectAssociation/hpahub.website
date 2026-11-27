package migrations

import (
	"errors"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addLongTermStatus, nil) }
func addLongTermStatus(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	status, ok := projects.Fields.GetByName("execution_status").(*core.SelectField)
	if !ok {
		return errors.New("project execution status field missing")
	}
	for _, value := range status.Values {
		if value == "long_term" {
			return nil
		}
	}
	status.Values = append(status.Values, "long_term")
	return app.Save(projects)
}
