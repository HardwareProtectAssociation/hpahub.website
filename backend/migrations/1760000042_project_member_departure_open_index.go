package migrations

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() { migrations.Register(fixProjectMemberDepartureOpenIndex, nil) }

func fixProjectMemberDepartureOpenIndex(app core.App) error {
	requests, err := app.FindCollectionByNameOrId("project_member_departure_requests")
	if err != nil {
		return err
	}
	indexes := make([]string, 0, len(requests.Indexes))
	for _, index := range requests.Indexes {
		if !strings.Contains(index, "idx_project_departures_open_member") {
			indexes = append(indexes, index)
		}
	}
	requests.Indexes = append(indexes, "CREATE UNIQUE INDEX idx_project_departures_open_member ON project_member_departure_requests (open_request_key) WHERE open_request_key != ''")
	return app.Save(requests)
}
