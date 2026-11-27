package migrations

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

const projectNumberRange = 90000

func init() { migrations.Register(addProjectNumbers, nil) }

func addProjectNumbers(app core.App) error {
	projects, err := app.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}
	if projects.Fields.GetByName("project_number") == nil {
		projects.Fields.Add(&core.TextField{Name: "project_number", Max: 9})
		if err := app.Save(projects); err != nil {
			return err
		}
	}

	rows, err := app.FindRecordsByFilter("projects", "", "id", 0, 0)
	if err != nil {
		return err
	}
	used := make(map[string]bool, len(rows))
	for _, project := range rows {
		number := project.GetString("project_number")
		if !validProjectNumber(number) || used[number] {
			number, err = nextMigrationProjectNumber(used)
			if err != nil {
				return err
			}
			project.Set("project_number", number)
			if err := app.Save(project); err != nil {
				return err
			}
		}
		used[number] = true
	}

	for _, index := range projects.Indexes {
		if strings.Contains(index, "idx_projects_project_number") {
			return nil
		}
	}
	projects.Indexes = append(projects.Indexes, "CREATE UNIQUE INDEX idx_projects_project_number ON projects (project_number) WHERE project_number != ''")
	return app.Save(projects)
}

func validProjectNumber(value string) bool {
	if len(value) != 9 || !strings.HasPrefix(value, "HPA-") {
		return false
	}
	number, err := strconv.Atoi(value[4:])
	return err == nil && number >= 10000 && number <= 99999
}

func nextMigrationProjectNumber(used map[string]bool) (string, error) {
	start, err := rand.Int(rand.Reader, big.NewInt(projectNumberRange))
	if err != nil {
		return "", err
	}
	for offset := int64(0); offset < projectNumberRange; offset++ {
		number := fmt.Sprintf("HPA-%05d", (start.Int64()+offset)%projectNumberRange+10000)
		if !used[number] {
			return number, nil
		}
	}
	return "", fmt.Errorf("all project numbers are already assigned")
}
