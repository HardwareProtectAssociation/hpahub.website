package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addMessaging, nil)
}

// addMessaging creates the in-app messaging system for user-to-user messages
// and system notifications (project approvals, etc.).
func addMessaging(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	messages := coNew("messages",
		coRelation("sender_user_id", users.Id, false), // null for system messages
		coRelation("recipient_user_id", users.Id, true),
		coText("subject", 200, false),
		coLongText("content", true),
		coSelect("category", true, "user", "system", "project_approval"),
		coRelation("project_id", projects.Id, false),
		coBool("read"),
		coDate("read_at", false),
	)
	coIndexes(messages,
		"CREATE INDEX idx_messages_recipient_read ON messages (recipient_user_id, read, created)",
		"CREATE INDEX idx_messages_sender ON messages (sender_user_id, created)",
		"CREATE INDEX idx_messages_project ON messages (project_id, category)",
	)
	return app.Save(messages)
}
