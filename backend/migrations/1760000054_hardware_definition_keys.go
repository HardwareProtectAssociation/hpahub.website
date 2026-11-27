package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(addHardwareDefinitionKeys, nil) }

// Keys address values in hardware.data, so groups cannot reuse a key within a type.
func addHardwareDefinitionKeys(app core.App) error {
	return app.RunInTransaction(func(tx core.App) error {
		for _, name := range []string{"hw_fields", "hw_tables"} {
			var duplicates int
			if err := tx.DB().NewQuery("SELECT COUNT(*) FROM (SELECT type, key FROM " + name + " GROUP BY type, key HAVING COUNT(*) > 1)").Row(&duplicates); err != nil {
				return err
			}
			if duplicates > 0 {
				return fmt.Errorf("%s has %d duplicate type/key groups; resolve them before upgrading (no records were removed)", name, duplicates)
			}
			collection, err := tx.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			index := "idx_" + name + "_type_key"
			if !containsIndex(collection.Indexes, index) {
				collection.Indexes = append(collection.Indexes, "CREATE UNIQUE INDEX "+index+" ON "+name+" (type, key)")
				if err := tx.Save(collection); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
