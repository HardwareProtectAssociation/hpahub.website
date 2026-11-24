package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil { return err }
		users.Fields.Add(&core.SelectField{Name: "role", Values: []string{"admin", "member", "user"}, MaxSelect: 1})
		users.Fields.GetByName("email").(*core.EmailField).Required = false
		users.CreateRule = nil
		users.UpdateRule = nil
		users.DeleteRule = nil
		owner := "@request.auth.id = id && @request.auth.collectionName = 'users'"
		users.ListRule = &owner
		users.ViewRule = &owner
		users.OAuth2.Enabled = false
		if err := app.Save(users); err != nil { return err }
		_, err = app.DB().NewQuery("UPDATE users SET role = 'member'").Execute()
		return err
	}, nil)
}
