package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectMemberDepartures, nil) }

func addProjectMemberDepartures(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if _, err := app.FindCollectionByNameOrId("project_member_departure_requests"); err == nil {
		return nil
	}
	requests := core.NewBaseCollection("project_member_departure_requests")
	requests.Fields.Add(
		&core.RelationField{Name: "project_id", CollectionId: projects.Id, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "member_user_id", CollectionId: users.Id, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "requester_user_id", CollectionId: users.Id, MaxSelect: 1, Required: true},
		&core.SelectField{Name: "request_type", Values: []string{"leave", "remove"}, MaxSelect: 1, Required: true},
		&core.TextField{Name: "reason", Required: true, Max: 2000},
		&core.SelectField{Name: "status", Values: []string{"pending_owner", "pending_member", "pending_admin", "approved", "rejected"}, MaxSelect: 1, Required: true},
		&core.DateField{Name: "expires_at", Required: true},
		&core.DateField{Name: "escalated_at"},
		&core.RelationField{Name: "reviewer_user_id", CollectionId: users.Id, MaxSelect: 1},
		&core.DateField{Name: "reviewed_at"},
		&core.TextField{Name: "review_note", Max: 2000},
		&core.TextField{Name: "open_request_key", Max: 200},
		&core.DateField{Name: "resolved_at"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	requests.Indexes = append(requests.Indexes,
		"CREATE UNIQUE INDEX idx_project_departures_open_member ON project_member_departure_requests (open_request_key)",
		"CREATE INDEX idx_project_departures_status_expiry ON project_member_departure_requests (status, expires_at)",
		"CREATE INDEX idx_project_departures_project_status ON project_member_departure_requests (project_id, status, created)",
	)
	return app.Save(requests)
}
