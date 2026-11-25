package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createSkillLikes, nil)
}

// createSkillLikes stores one endorsement per member and user skill. The
// unique index prevents duplicate likes; counts are derived from this table.
func createSkillLikes(app core.App) error {
	userSkills, err := app.FindCollectionByNameOrId("user_skills")
	if err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	likes := coNew("user_skill_likes",
		coRelation("user_skill_id", userSkills.Id, true),
		coRelation("user_id", users.Id, true),
	)
	if field, ok := likes.Fields.GetByName("user_skill_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	if field, ok := likes.Fields.GetByName("user_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	coIndexes(likes,
		"CREATE UNIQUE INDEX idx_user_skill_likes_unique ON user_skill_likes (user_skill_id, user_id)",
		"CREATE INDEX idx_user_skill_likes_skill ON user_skill_likes (user_skill_id)",
	)
	return app.Save(likes)
}
