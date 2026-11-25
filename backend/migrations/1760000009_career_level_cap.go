package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(extendCareerLevels, nil)
}

// extendCareerLevels fills every career up to coCareerLevelCap with the
// original example curve extended quadratically:
//
//	xp(L) = 50 * L * (L-1)
//
// Levels 1-5 therefore stay 0/100/300/600/1000, level 10 is 4,500 and level
// 100 is 495,000. Keeping the thresholds in the database leaves room to tune
// the curve or raise the cap with a later migration.
func extendCareerLevels(app core.App) error {
	_, err := app.DB().NewQuery(`
		INSERT INTO career_level_thresholds
			(id, career_identity_id, level_number, minimum_total_xp, display_title, created, updated)
		SELECT
			substr(lower(hex(randomblob(8))), 1, 15),
			ci.id,
			levels.n,
			50 * levels.n * (levels.n - 1),
			'Lv.' || levels.n,
			strftime('%Y-%m-%d %H:%M:%fZ', 'now'),
			strftime('%Y-%m-%d %H:%M:%fZ', 'now')
		FROM collaboration_identities ci
		JOIN (
			WITH RECURSIVE levels(n) AS (
				SELECT 1
				UNION ALL
				SELECT n + 1 FROM levels WHERE n < {:cap}
			)
			SELECT n FROM levels
		) levels ON 1 = 1
		WHERE NOT EXISTS (
			SELECT 1 FROM career_level_thresholds t
			WHERE t.career_identity_id = ci.id AND t.level_number = levels.n
		)
	`).Bind(dbx.Params{"cap": coCareerLevelCap}).Execute()
	return err
}
