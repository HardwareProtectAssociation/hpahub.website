package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addMemberRelations, nil)
}

// addMemberRelations stores one directed relationship per member pair.
func addMemberRelations(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	relations := coNew("member_relations",
		coRelation("actor_user_id", users.Id, true),
		coRelation("target_user_id", users.Id, true),
		coSelect("relation", true, "admire", "watch", "investigate"),
	)
	coIndexes(relations,
		"CREATE UNIQUE INDEX idx_member_relations_actor_target ON member_relations (actor_user_id, target_user_id)",
		"CREATE INDEX idx_member_relations_target_relation ON member_relations (target_user_id, relation)",
	)
	return app.Save(relations)
}
