package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(clearHardwareData, nil)
}

// clearHardwareData empties the hardware database (hardware, hardware_media and
// hardware_options) so the renamed hardware database starts without legacy data.
func clearHardwareData(app core.App) error {
	for _, table := range []string{"hardware_media", "hardware_options", "hardware"} {
		if _, err := app.DB().NewQuery("DELETE FROM " + table).Execute(); err != nil {
			return err
		}
	}
	return nil
}
