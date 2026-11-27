package migrations

import (
    "github.com/pocketbase/pocketbase/core"
    "github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectSelectionDetails, nil) }

func addProjectSelectionDetails(app core.App) error {
    projects, err := app.FindCollectionByNameOrId("projects")
    if err != nil { return err }
    changed := false
    for _, name := range []string{"project_need_details", "provider_offer_details"} {
        if projects.Fields.GetByName(name) == nil {
            projects.Fields.Add(&core.JSONField{Name: name, MaxSize: 32768})
            changed = true
        }
    }
    if changed { return app.Save(projects) }
    return nil
}
