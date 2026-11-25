package main

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type hardwareInput struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Brand     string         `json:"brand"`
	Subtitle  string         `json:"subtitle"`
	SortOrder float64        `json:"sort_order"`
	Data      map[string]any `json:"data"`
}

func recordJSONField(r *core.Record, name string) any {
	var value any
	b, _ := json.Marshal(r.Get(name))
	_ = json.Unmarshal(b, &value)
	return value
}

func hardwareData(r *core.Record) map[string]any {
	data, _ := recordJSONField(r, "data").(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	return data
}

// hardwareSchema assembles the data-driven hardware structure (types with
// their field groups and tables) from hw_types, hw_fields and hw_tables.
func hardwareSchema(app core.App) (map[string]any, error) {
	typeRecords, err := app.FindRecordsByFilter("hw_types", "", "sort_order,id", 0, 0)
	if err != nil {
		return nil, err
	}
	fieldRecords, err := app.FindRecordsByFilter("hw_fields", "", "sort_order,id", 0, 0)
	if err != nil {
		return nil, err
	}
	tableRecords, err := app.FindRecordsByFilter("hw_tables", "", "sort_order,id", 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(typeRecords))
	for _, t := range typeRecords {
		var groups []map[string]any
		groupIndex := map[string]int{}
		for _, f := range fieldRecords {
			if f.GetString("type") != t.Id {
				continue
			}
			title := f.GetString("group")
			i, ok := groupIndex[title]
			if !ok {
				i = len(groups)
				groupIndex[title] = i
				groups = append(groups, map[string]any{"key": title, "fields": []any{}})
			}
			groups[i]["fields"] = append(groups[i]["fields"].([]any), map[string]any{"key": f.GetString("key"), "label": f.GetString("label")})
		}
		var tables []map[string]any
		for _, r := range tableRecords {
			if r.GetString("type") != t.Id {
				continue
			}
			tables = append(tables, map[string]any{"key": r.GetString("key"), "title": r.GetString("title"), "columns": recordJSONField(r, "columns")})
		}
		out = append(out, map[string]any{
			"key": t.GetString("key"), "label": t.GetString("label"),
			"name_field": t.GetString("name_field"), "subtitle_fields": t.GetString("subtitle_fields"),
			"groups": groups, "tables": tables,
		})
	}
	return map[string]any{"types": out}, nil
}

func hardwareView(r *core.Record) map[string]any {
	return map[string]any{
		"id": r.Id, "kind": r.GetString("kind"), "name": r.GetString("name"),
		"brand": r.GetString("brand"), "subtitle": r.GetString("subtitle"),
		"sort_order": r.GetFloat("sort_order"), "data": hardwareData(r),
	}
}

func hardwarePrivateKey(v any) bool {
	switch value := v.(type) {
	case map[string]any:
		for k, child := range value {
			if k == "related_media" || hardwarePrivateKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if hardwarePrivateKey(child) {
				return true
			}
		}
	}
	return false
}

// hardwareType returns the hw_types record for a hardware kind, i.e. the
// data-driven definition of which fields and tables that kind uses.
func hardwareType(app core.App, kind string) (*core.Record, error) {
	rs, err := app.FindRecordsByFilter("hw_types", "key = {:key}", "", 1, 0, dbx.Params{"key": kind})
	if err != nil || len(rs) == 0 {
		return nil, fmt.Errorf("invalid hardware kind")
	}
	return rs[0], nil
}

// hardwareNameField returns the field key holding the display name of a kind.
func hardwareNameField(app core.App, kind string) string {
	if r, err := hardwareType(app, kind); err == nil {
		if key := r.GetString("name_field"); key != "" {
			return key
		}
	}
	return "name"
}

func saveHardware(app core.App, r *core.Record, in hardwareInput) error {
	if _, err := hardwareType(app, in.Kind); err != nil {
		return err
	}
	if in.Data == nil || hardwarePrivateKey(in.Data) {
		return fmt.Errorf("invalid hardware data")
	}
	r.Set("kind", in.Kind)
	r.Set("name", in.Name)
	r.Set("brand", in.Brand)
	r.Set("subtitle", in.Subtitle)
	r.Set("sort_order", in.SortOrder)
	r.Set("data", in.Data)
	return app.Save(r)
}
