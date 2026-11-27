package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addBusinessQueryIndexes, nil) }

// Match the filters and ordering used by the project, report and inbox pages.
var businessQueryIndexes = []struct{ collection, name, columns string }{
	{"projects", "idx_projects_public_feed", "review_status, deleted_at, published_at DESC, updated DESC, id DESC"},
	{"projects", "idx_projects_owner_feed", "owner_user_id, deleted_at, updated DESC"},
	{"project_members", "idx_project_members_project_status_joined", "project_id, status, joined_at"},
	{"project_follows", "idx_project_follows_user_active", "user_id, is_active"},
	{"project_completion_requests", "idx_project_completions_project_status_created", "project_id, status, created"},
	{"project_completion_requests", "idx_project_completions_user_status_created", "user_id, status, created"},
	{"project_join_requests", "idx_project_join_requests_status_created", "status, created, id"},
	{"hardware", "idx_hardware_public_order", "is_public, sort_order, name"},
	{"hardware", "idx_hardware_kind_public_order", "kind, is_public, sort_order, name"},
	{"hardware", "idx_hardware_submitter", "submitted_by"},
	{"hw_fields", "idx_hw_fields_type_order", "type, sort_order, id"},
	{"hw_tables", "idx_hw_tables_type_order", "type, sort_order, id"},
	{"messages", "idx_messages_recipient_created", "recipient_user_id, created DESC, id DESC"},
	{"user_titles", "idx_user_titles_user_revoked_created", "user_id, revoked_at, created, id"},
}

func addBusinessQueryIndexes(app core.App) error {
	return app.RunInTransaction(func(tx core.App) error {
		for _, index := range businessQueryIndexes {
			collection, err := tx.FindCollectionByNameOrId(index.collection)
			if err != nil {
				return err
			}
			if containsIndex(collection.Indexes, index.name) {
				continue
			}
			collection.Indexes = append(collection.Indexes, "CREATE INDEX "+index.name+" ON "+index.collection+" ("+index.columns+")")
			if err := tx.Save(collection); err != nil {
				return err
			}
		}
		return nil
	})
}
