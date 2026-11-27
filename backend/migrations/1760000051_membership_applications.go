package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addMembershipApplications, nil) }

func addMembershipApplications(app core.App) error {
	if _, err := app.FindCollectionByNameOrId("membership_applications"); err == nil {
		return nil
	}
	collection := coNew("membership_applications",
		coText("email", 255, true),
		coText("applicant_name", 80, true),
		coLongText("reason", true),
		coSelect("status", true, "pending", "approved", "rejected"),
		coText("reviewer_user_id", 20, false),
		coText("invitation_code", 32, false),
		coNumber("invitation_expires", false, true, nil, nil),
		&core.DateField{Name: "reviewed_at"},
	)
	coIndexes(collection,
		`CREATE UNIQUE INDEX idx_membership_applications_pending_email ON membership_applications (lower(email)) WHERE status = 'pending'`,
		`CREATE INDEX idx_membership_applications_status_created ON membership_applications (status, created DESC)`,
	)
	return app.Save(collection)
}
