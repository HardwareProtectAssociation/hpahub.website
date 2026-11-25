package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addHardwareNews, nil)
}

// addHardwareNews stores member-posted hardware news and one reaction per
// member per post ("like" or "useless").
func addHardwareNews(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	news := coNew("hardware_news",
		coRelation("author_user_id", users.Id, true),
		coText("title", 160, true),
		coLongText("content", true),
		coText("link", 500, false),
	)
	if err := app.Save(news); err != nil {
		return err
	}
	reactions := coNew("hardware_news_reactions",
		coRelation("news_id", news.Id, true),
		coRelation("user_id", users.Id, true),
		coSelect("reaction", true, "like", "useless"),
	)
	if field, ok := reactions.Fields.GetByName("news_id").(*core.RelationField); ok {
		field.CascadeDelete = true
	}
	coIndexes(reactions,
		"CREATE UNIQUE INDEX idx_hardware_news_reactions_news_user ON hardware_news_reactions (news_id, user_id)",
		"CREATE INDEX idx_hardware_news_reactions_news ON hardware_news_reactions (news_id, reaction)",
	)
	return app.Save(reactions)
}
