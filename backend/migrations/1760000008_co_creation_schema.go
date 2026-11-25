package migrations

import "github.com/pocketbase/pocketbase/migrations"

func init() {
	migrations.Register(buildCoCreationSchema, nil)
}

// The build function lives in co_creation_schema*.go so the migration entry
// stays a single registered version (1760000008) for the whole schema.
