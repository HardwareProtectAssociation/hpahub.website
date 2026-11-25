package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addHealthCheck, nil)
}

// addHealthCheck adds the current availability status, the daily check-in
// preference and the per-day health check-in records.
func addHealthCheck(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.Fields.Add(
		&core.SelectField{Name: "current_status", Values: []string{"busy", "free"}, MaxSelect: 1},
		&core.BoolField{Name: "daily_health_enabled"},
	)
	if err := app.Save(users); err != nil {
		return err
	}
	if _, err := app.DB().NewQuery("UPDATE users SET daily_health_enabled = 1").Execute(); err != nil {
		return err
	}
	checkins := coNew("health_checkins",
		coRelation("user_id", users.Id, true),
		coText("check_date", 10, true),
		coSelect("mood", true, "great", "good", "okay", "low", "bad"),
		coBool("rested"),
		coBool("exercised"),
		coBool("ate_on_time"),
		coText("note", 300, false),
	)
	coIndexes(checkins,
		"CREATE UNIQUE INDEX idx_health_checkins_user_date ON health_checkins (user_id, check_date)",
	)
	return app.Save(checkins)
}
