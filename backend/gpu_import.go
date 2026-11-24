package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// importGPU copies a consistent read-only legacy snapshot into the main database.
func importGPU(app core.App, source string) error {
	path, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return err
	}
	defer db.Close()
	snapshot, err := db.Begin()
	if err != nil {
		return err
	}
	defer snapshot.Rollback()
	var version int
	if err := snapshot.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version < 0 || version > 1 {
		return fmt.Errorf("unsupported GPU database version: %d", version)
	}

	tables := []struct {
		name string
		keys []string
	}{
		{"gpus", []string{"id"}}, {"cooling", []string{"gpu_id"}},
		{"mark3d", []string{"gpu_id"}}, {"blender", []string{"gpu_id"}},
		{"sw", []string{"gpu_id"}}, {"productivity", []string{"id"}},
		{"games", []string{"id"}}, {"codec_runs", []string{"id"}},
		{"items", []string{"id"}}, {"gpu_custom", []string{"gpu_id", "section", "field_key"}},
		{"option_values", []string{"kind", "value"}},
	}
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	return app.RunInTransaction(func(target core.App) error {
		for _, table := range tables {
			var exists int
			if err := snapshot.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table.name).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				if version == 0 && table.name != "gpus" {
					continue
				}
				return fmt.Errorf("GPU source table %s is missing", table.name)
			}
			rows, err := snapshot.Query("SELECT * FROM " + quote(table.name))
			if err != nil {
				return err
			}
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			quoted, placeholders, comparisons := make([]string, len(cols)), make([]string, len(cols)), make([]string, len(cols))
			keyConditions := make([]string, 0, len(table.keys))
			for i, col := range cols {
				quoted[i] = quote(col)
				placeholders[i] = fmt.Sprintf("{:v%d}", i)
				comparisons[i] = quoted[i] + " IS " + placeholders[i]
				for _, key := range table.keys {
					if col == key {
						keyConditions = append(keyConditions, comparisons[i])
					}
				}
			}
			if len(keyConditions) != len(table.keys) {
				rows.Close()
				return fmt.Errorf("GPU source table %s is missing key columns", table.name)
			}
			// Prepare the column references even for an empty source table.
			var dummy int
			qualified := make([]string, len(quoted))
			for i, col := range quoted {
				qualified[i] = "t." + col
			}
			if err := target.DB().NewQuery("SELECT count(*) FROM " + quote(table.name) + " AS t WHERE 0 AND (" + strings.Join(qualified, " IS NULL AND ") + " IS NULL)").Row(&dummy); err != nil {
				rows.Close()
				return fmt.Errorf("GPU table %s schema: %w", table.name, err)
			}
			for rows.Next() {
				values := make([]any, len(cols))
				pointers := make([]any, len(cols))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err := rows.Scan(pointers...); err != nil {
					rows.Close()
					return err
				}
				params := dbx.Params{}
				for i, value := range values {
					params[fmt.Sprintf("v%d", i)] = value
				}
				var count int
				if err := target.DB().NewQuery("SELECT count(*) FROM " + quote(table.name) + " WHERE " + strings.Join(keyConditions, " AND ")).Bind(params).Row(&count); err != nil {
					rows.Close()
					return err
				}
				if count != 0 {
					if err := target.DB().NewQuery("SELECT count(*) FROM " + quote(table.name) + " WHERE " + strings.Join(comparisons, " AND ")).Bind(params).Row(&count); err != nil {
						rows.Close()
						return err
					}
					if count != 1 {
						rows.Close()
						return fmt.Errorf("GPU import conflict in %s", table.name)
					}
					continue
				}
				_, err = target.DB().NewQuery("INSERT INTO " + quote(table.name) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")").Bind(params).Execute()
				if err != nil {
					rows.Close()
					return fmt.Errorf("import GPU table %s: %w", table.name, err)
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		var sequenceExists int
		if err := snapshot.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'sqlite_sequence'").Scan(&sequenceExists); err != nil {
			return err
		}
		if sequenceExists != 0 {
			rows, err := snapshot.Query("SELECT name, seq FROM sqlite_sequence")
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var name string
				var seq int64
				if err := rows.Scan(&name, &seq); err != nil {
					return err
				}
				allowed := false
				for _, table := range []string{"gpus", "productivity", "games", "codec_runs", "items"} {
					if name == table {
						allowed = true
						break
					}
				}
				if !allowed {
					continue
				}
				if seq < 0 {
					return fmt.Errorf("invalid GPU sequence for %s", name)
				}
				params := dbx.Params{"name": name, "seq": seq}
				if _, err := target.DB().NewQuery("UPDATE sqlite_sequence SET seq = max(seq, {:seq}) WHERE name = {:name}").Bind(params).Execute(); err != nil {
					return err
				}
				if _, err := target.DB().NewQuery("INSERT INTO sqlite_sequence (name, seq) SELECT {:name}, {:seq} WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = {:name})").Bind(params).Execute(); err != nil {
					return err
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
		}
		_, err := target.DB().NewQuery("INSERT OR IGNORE INTO hpahub_gpu_import (id) VALUES (1)").Execute()
		return err
	})
}
