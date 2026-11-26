package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(protectLegacyHardware, nil)
}

// This guard runs before the deployed destructive migration on older databases.
func protectLegacyHardware(app core.App) error {
	var applied int
	if err := app.DB().NewQuery(`SELECT count(*) FROM _migrations WHERE file = '1760000006_clear_hardware_data.go'`).Row(&applied); err != nil {
		return err
	}
	if applied != 0 {
		return nil
	}
	for _, table := range []string{"hardware", "hardware_media", "hardware_options"} {
		var count int
		if err := app.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("legacy hardware data exists in %s; back up the database and migrate the data before upgrading", table)
		}
	}
	return nil
}
