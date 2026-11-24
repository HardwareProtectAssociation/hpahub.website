package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// importGPU imports one consistent read-only snapshot into empty collections.
func importGPU(app core.App, source string) error {
	path, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	urlPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && len(urlPath) > 0 && urlPath[0] != '/' {
		urlPath = "/" + urlPath
	}
	u := url.URL{Scheme: "file", Path: urlPath, RawQuery: "mode=ro"}
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
	names := []string{"gpus", "cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "items", "gpu_custom", "option_values"}
	tables := make(map[string][]map[string]any, len(names))
	for _, name := range names {
		var exists int
		if err := snapshot.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			if version == 0 && name != "gpus" {
				continue
			}
			return fmt.Errorf("GPU source table %s is missing", name)
		}
		rows, err := snapshot.Query("SELECT * FROM \"" + name + "\"")
		if err != nil {
			return err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return err
		}
		tables[name] = []map[string]any{}
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return err
			}
			row := make(map[string]any, len(columns))
			for i, column := range columns {
				if b, ok := values[i].([]byte); ok {
					values[i] = string(b)
				}
				row[column] = values[i]
			}
			tables[name] = append(tables[name], row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return app.RunInTransaction(func(tx core.App) error {
		for _, table := range []string{"hardware", "hardware_media", "hardware_options"} {
			var count int
			if err := tx.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("GPU import conflict: %s is not empty", table)
			}
		}
		hardware, err := tx.FindCollectionByNameOrId("hardware")
		if err != nil {
			return err
		}
		media, err := tx.FindCollectionByNameOrId("hardware_media")
		if err != nil {
			return err
		}
		options, err := tx.FindCollectionByNameOrId("hardware_options")
		if err != nil {
			return err
		}
		gpus := map[any]*core.Record{}
		data := map[any]map[string]any{}
		paths := map[any][]any{}
		for _, row := range tables["gpus"] {
			id, ok := row["id"]
			if !ok || id == nil || gpus[id] != nil {
				return fmt.Errorf("invalid GPU source id")
			}
			fields := map[string]any{}
			for key, value := range row {
				switch key {
				case "id", "related_media", "created_at", "updated_at", "sort_order":
				default:
					fields[key] = value
				}
			}
			sections := map[string]any{}
			lists := map[string]any{"productivity": []any{}, "games": []any{}, "encoding": []any{}, "decoding": []any{}}
			custom := map[string]any{}
			for _, section := range []string{"basic", "cooling", "mark3d"} {
				custom[section] = map[string]any{"hidden": []any{}, "extra": []any{}}
			}
			data[id] = map[string]any{"fields": fields, "sections": sections, "rows": lists, "custom": custom}
			record := core.NewRecord(hardware)
			record.Set("kind", "gpu")
			record.Set("name", row["model"])
			record.Set("brand", row["brand"])
			record.Set("sort_order", row["sort_order"])
			gpus[id] = record
			paths[id] = []any{}
			if value, ok := row["related_media"]; ok && value != nil {
				var imported []any
				if err := json.Unmarshal([]byte(fmt.Sprint(value)), &imported); err != nil {
					return fmt.Errorf("GPU %v related_media: %w", id, err)
				}
				paths[id] = imported
			}
		}
		for _, name := range []string{"cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "gpu_custom"} {
			rows := tables[name]
			sort.SliceStable(rows, func(i, j int) bool {
				a, aok := rows[i]["sort_order"].(int64)
				b, bok := rows[j]["sort_order"].(int64)
				if aok && bok && a != b {
					return a < b
				}
				ai, aok := rows[i]["id"].(int64)
				bi, bok := rows[j]["id"].(int64)
				return aok && bok && ai < bi
			})
			for _, row := range rows {
				id := row["gpu_id"]
				d := data[id]
				if d == nil {
					return fmt.Errorf("orphan GPU row in %s", name)
				}
				delete(row, "gpu_id")
				delete(row, "id")
				switch name {
				case "cooling", "mark3d", "blender", "sw":
					sections := d["sections"].(map[string]any)
					if _, exists := sections[name]; exists {
						return fmt.Errorf("duplicate GPU section %s", name)
					}
					sections[name] = row
				case "gpu_custom":
					custom := d["custom"].(map[string]any)
					section, ok := row["section"].(string)
					if !ok {
						return fmt.Errorf("invalid GPU custom section")
					}
					group, ok := custom[section].(map[string]any)
					if !ok {
						group = map[string]any{"hidden": []any{}, "extra": []any{}}
						custom[section] = group
					}
					// Array order replaces the old row identifier and sort order.
					if row["hidden"] != nil && row["hidden"] != int64(0) {
						group["hidden"] = append(anySlice(group["hidden"]), row["field_key"])
					} else {
						extra := map[string]any{}
						for k, v := range row {
							extra[k] = v
						}
						delete(extra, "section")
						delete(extra, "field_key")
						delete(extra, "hidden")
						delete(extra, "sort_order")
						extra["key"] = row["field_key"]
						group["extra"] = append(anySlice(group["extra"]), extra)
					}
				default:
					key := name
					if name == "codec_runs" {
						key, _ = row["kind"].(string)
						if key != "encoding" && key != "decoding" {
							return fmt.Errorf("invalid GPU codec kind %q", key)
						}
					}
					lists := d["rows"].(map[string]any)
					lists[key] = append(anySlice(lists[key]), row)
				}
			}
		}
		for _, row := range tables["gpus"] {
			id := row["id"]
			record := gpus[id]
			private := paths[id]
			record.Set("data", gpuImportPrivate(data[id], []any{}, &private))
			paths[id] = private
			if err := tx.Save(record); err != nil {
				return err
			}
			m := core.NewRecord(media)
			m.Set("hardware", record.Id)
			m.Set("paths", paths[id])
			if err := tx.Save(m); err != nil {
				return err
			}
		}
		for _, row := range tables["items"] {
			kind, _ := row["kind"].(string)
			if kind != "cpu" && kind != "device" {
				return fmt.Errorf("invalid hardware item kind %q", kind)
			}
			itemData := map[string]any{}
			private := []any{}
			for _, key := range []string{"fields", "rows"} {
				var value any = map[string]any{}
				if raw, exists := row[key]; exists && raw != nil {
					if !json.Valid([]byte(fmt.Sprint(raw))) {
						return fmt.Errorf("hardware item %v %s: invalid JSON", row["id"], key)
					}
					decoder := json.NewDecoder(strings.NewReader(fmt.Sprint(raw)))
					decoder.UseNumber()
					if err := decoder.Decode(&value); err != nil {
						return fmt.Errorf("hardware item %v %s: %w", row["id"], key, err)
					}
				} else if exists {
					value = nil
				}
				itemData[key] = gpuImportPrivate(value, []any{key}, &private)
			}
			record := core.NewRecord(hardware)
			for _, key := range []string{"kind", "name", "brand", "subtitle", "sort_order"} {
				record.Set(key, row[key])
			}
			record.Set("data", itemData)
			if err := tx.Save(record); err != nil {
				return err
			}
			if len(private) > 0 {
				m := core.NewRecord(media)
				m.Set("hardware", record.Id)
				m.Set("paths", private)
				if err := tx.Save(m); err != nil {
					return err
				}
			}
		}
		for _, row := range tables["option_values"] {
			record := core.NewRecord(options)
			for _, key := range []string{"kind", "value", "sort_order"} {
				record.Set(key, row[key])
			}
			if err := tx.Save(record); err != nil {
				return err
			}
		}
		return nil
	})
}

func anySlice(value any) []any {
	if value == nil {
		return nil
	}
	return value.([]any)
}

// Nested media keeps its original location and value inside the private record.
func gpuImportPrivate(value any, path []any, private *[]any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			next := append(append([]any{}, path...), key)
			if key == "related_media" {
				*private = append(*private, map[string]any{"path": next, "value": child})
				delete(v, key)
			} else {
				v[key] = gpuImportPrivate(child, next, private)
			}
		}
	case []any:
		for i, child := range v {
			next := append(append([]any{}, path...), i)
			v[i] = gpuImportPrivate(child, next, private)
		}
	}
	return value
}
