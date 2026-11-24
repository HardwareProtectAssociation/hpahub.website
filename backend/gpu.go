package main

import (
	"encoding/json"
	"fmt"

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

func hardwareData(r *core.Record) map[string]any {
	var data map[string]any
	b, _ := json.Marshal(r.Get("data"))
	_ = json.Unmarshal(b, &data)
	if data == nil {
		data = map[string]any{}
	}
	return data
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

func saveHardware(app core.App, r *core.Record, in hardwareInput) error {
	if in.Kind != "gpu" && in.Kind != "cpu" && in.Kind != "device" {
		return fmt.Errorf("invalid hardware kind")
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
