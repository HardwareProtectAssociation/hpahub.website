package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMediaLimit(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	for _, name := range []string{"member_cards", "showcase_projects", "user_skills"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := collection.Fields.GetByName("images").(*core.FileField).MaxSize; got != 2<<20 {
			t.Errorf("%s image limit = %d", name, got)
		}
	}
}

func TestAvatarThumbnailField(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := users.Fields.GetByName("avatar_thumb").(*core.FileField)
	if !ok || !field.Protected || len(field.MimeTypes) != 1 || field.MimeTypes[0] != "image/webp" {
		t.Fatalf("invalid avatar thumbnail field: %#v", field)
	}
}
