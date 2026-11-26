package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addSkillFeatured, nil)
}

// addSkillFeatured marks the skills a member chooses to show on the public
// profile. The member may feature any number of skills.
func addSkillFeatured(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("user_skills")
	if err != nil {
		return err
	}
	collection.Fields.Add(&core.BoolField{Name: "featured"})
	return app.Save(collection)
}
