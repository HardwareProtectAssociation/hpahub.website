package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addHardwareReviews, nil)
}

// addHardwareReviews stores member reviews of hardware items and one like per
// member per review. Home page hot hardware ranks by these likes.
func addHardwareReviews(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	hardware, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		return err
	}
	reviews := coNew("hardware_reviews",
		coRelation("hardware_id", hardware.Id, true),
		coRelation("author_user_id", users.Id, true),
		coText("title", 160, true),
		coLongText("content", true),
	)
	if field, ok := reviews.Fields.GetByName("hardware_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	coIndexes(reviews,
		"CREATE INDEX idx_hardware_reviews_hardware ON hardware_reviews (hardware_id, created)",
		"CREATE INDEX idx_hardware_reviews_author ON hardware_reviews (author_user_id)",
	)
	if err := app.Save(reviews); err != nil {
		return err
	}
	likes := coNew("hardware_review_likes",
		coRelation("review_id", reviews.Id, true),
		coRelation("user_id", users.Id, true),
	)
	if field, ok := likes.Fields.GetByName("review_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	if field, ok := likes.Fields.GetByName("user_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	coIndexes(likes,
		"CREATE UNIQUE INDEX idx_hardware_review_likes_unique ON hardware_review_likes (review_id, user_id)",
		"CREATE INDEX idx_hardware_review_likes_review ON hardware_review_likes (review_id)",
	)
	return app.Save(likes)
}
