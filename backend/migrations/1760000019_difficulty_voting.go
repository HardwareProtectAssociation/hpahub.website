package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addDifficultyVoting, nil)
}

// addDifficultyVoting adds the difficulty voting mechanism:
// project owner proposes a difficulty, admins vote (approve/oppose),
// consensus requires >=3 admin approvals, 0 opposition, and owner confirmation.
func addDifficultyVoting(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	projects.Fields.Add(
		&core.SelectField{Name: "owner_proposed_difficulty", Values: []string{"D", "C", "B", "A", "S"}, MaxSelect: 1},
		&core.BoolField{Name: "owner_difficulty_confirmed"},
	)
	if err := app.Save(projects); err != nil {
		return err
	}

	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	votes := coNew("project_difficulty_votes",
		coRelation("project_id", projects.Id, true),
		coRelation("voter_user_id", users.Id, true),
		coSelect("difficulty", true, "D", "C", "B", "A", "S"),
		coSelect("stance", true, "approve", "oppose"),
		coText("comment", 500, false),
	)
	coIndexes(votes,
		"CREATE UNIQUE INDEX idx_project_difficulty_votes_unique ON project_difficulty_votes (project_id, voter_user_id)",
		"CREATE INDEX idx_project_difficulty_votes_project ON project_difficulty_votes (project_id, difficulty, stance)",
	)
	return app.Save(votes)
}
