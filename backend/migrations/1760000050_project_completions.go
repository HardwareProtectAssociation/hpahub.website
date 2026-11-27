package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectCompletions, nil) }

func addProjectCompletions(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	changed := false
	if projects.Fields.GetByName("is_repeatable") == nil {
		projects.Fields.Add(&core.BoolField{Name: "is_repeatable"})
		changed = true
	}
	if projects.Fields.GetByName("repeat_limit") == nil {
		projects.Fields.Add(&core.NumberField{Name: "repeat_limit", OnlyInt: true, Min: coFloat(1), Max: coFloat(9999)})
		changed = true
	}
	if changed {
		if err := app.Save(projects); err != nil {
			return err
		}
	}

	if _, err := app.FindCollectionByNameOrId("project_completion_requests"); err == nil {
		return nil
	}
	collection := coNew("project_completion_requests",
		coRelation("project_id", projects.Id, true),
		coRelation("user_id", users.Id, true),
		coNumber("completion_number", true, true, coFloat(1), coFloat(9999)),
		coLongText("message", true),
		coSelect("status", true, "pending", "approved", "rejected"),
		coRelation("reviewer_user_id", users.Id, false),
		&core.DateField{Name: "reviewed_at"},
	)
	coIndexes(collection,
		`CREATE UNIQUE INDEX idx_project_completion_pending ON project_completion_requests (project_id, user_id) WHERE status = 'pending'`,
		`CREATE UNIQUE INDEX idx_project_completion_approved_number ON project_completion_requests (project_id, user_id, completion_number) WHERE status = 'approved'`,
	)
	return app.Save(collection)
}
