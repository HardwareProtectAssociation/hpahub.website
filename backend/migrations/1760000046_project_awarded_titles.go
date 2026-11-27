package migrations

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addProjectAwardedTitles, nil) }

func addProjectAwardedTitles(app core.App) error {
	titles, err := app.FindCollectionByNameOrId("user_titles")
	if err != nil {
		return err
	}
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	changed := false
	if titles.Fields.GetByName("source_project_id") == nil {
		titles.Fields.Add(&core.RelationField{Name: "source_project_id", CollectionId: projects.Id, MaxSelect: 1})
		changed = true
	}
	indexes := make([]string, 0, len(titles.Indexes)+1)
	for _, index := range titles.Indexes {
		if strings.Contains(index, "idx_user_titles_user_name ON") {
			changed = true
			continue
		}
		indexes = append(indexes, index)
	}
	if !containsIndex(indexes, "idx_user_titles_user_name_project") {
		indexes = append(indexes, "CREATE UNIQUE INDEX idx_user_titles_user_name_project ON user_titles (user_id, name, source_project_id) WHERE revoked_at = ''")
		changed = true
	}
	if changed {
		titles.Indexes = indexes
		return app.Save(titles)
	}
	return nil
}

func containsIndex(indexes []string, name string) bool {
	for _, index := range indexes {
		if strings.Contains(index, name) {
			return true
		}
	}
	return false
}
