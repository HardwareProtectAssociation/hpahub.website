package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addUserHardwareConfig, nil)
}

// addUserHardwareConfig stores the hardware items a user personally uses.
func addUserHardwareConfig(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	configs := coNew("user_hardware_configs",
		coRelation("user_id", users.Id, true),
		coText("category", 50, true),
		coText("name", 200, true),
		coText("detail", 500, false),
		coNumber("sort_order", false, true, nil, nil),
	)
	coIndexes(configs,
		"CREATE INDEX idx_user_hardware_configs_user ON user_hardware_configs (user_id, sort_order)",
	)
	return app.Save(configs)
}
