package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addHardwareArticles, nil) }

// addHardwareArticles stores the editorial article separately from the hardware's structured test report.
func addHardwareArticles(app core.App) error {
	hardware, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if _, err := app.FindCollectionByNameOrId("hardware_articles"); err == nil {
		return nil
	}
	articles := core.NewBaseCollection("hardware_articles")
	articles.Fields.Add(&core.RelationField{Name: "hardware_id", CollectionId: hardware.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
	articles.Fields.Add(&core.RelationField{Name: "author_user_id", CollectionId: users.Id, MaxSelect: 1})
	articles.Fields.Add(&core.TextField{Name: "title", Required: true, Max: 160})
	articles.Fields.Add(&core.TextField{Name: "summary", Max: 1000})
	articles.Fields.Add(&core.JSONField{Name: "body", MaxSize: 1 << 20})
	articles.Fields.Add(&core.FileField{Name: "images", MaxSelect: 20, MaxSize: 5 << 20, MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"}})
	articles.Fields.Add(&core.BoolField{Name: "published"})
	articles.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	articles.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
	articles.Indexes = append(articles.Indexes,
		"CREATE INDEX idx_hardware_articles_hardware ON hardware_articles (hardware_id, published, updated)",
		"CREATE INDEX idx_hardware_articles_author ON hardware_articles (author_user_id, updated)",
	)
	return app.Save(articles)
}
