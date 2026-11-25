package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func hardwareBody(e *core.RequestEvent, value any) error {
	dec := json.NewDecoder(http.MaxBytesReader(e.Response, e.Request.Body, 8<<20))
	if err := dec.Decode(value); err != nil {
		return e.BadRequestError("Invalid JSON.", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return e.BadRequestError("Invalid JSON.", err)
	}
	return nil
}

func bindHardwareRoutes(s *core.ServeEvent) {
	base := "/api/hpahub/hardware"
	authorize := func(e *core.RequestEvent) error {
		noCache(e)
		return requireContentRole(e, false, false)
	}
	find := func(e *core.RequestEvent) (*core.Record, error) {
		r, err := e.App.FindRecordById("hardware", e.Request.PathValue("id"))
		if err != nil {
			return nil, e.NotFoundError("Hardware not found.", nil)
		}
		return r, nil
	}
	s.Router.GET(base+"/records", func(e *core.RequestEvent) error {
		noCache(e)
		rs, err := e.App.FindRecordsByFilter("hardware", "", "sort_order,id", 0, 0)
		if err != nil {
			return e.InternalServerError("Unable to load hardware.", err)
		}
		out := make([]map[string]any, 0, len(rs))
		for _, r := range rs {
			out = append(out, hardwareView(r))
		}
		return e.JSON(200, out)
	})
	s.Router.GET(base+"/records/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		r, err := find(e)
		if err != nil {
			return err
		}
		return e.JSON(200, hardwareView(r))
	})
	s.Router.POST(base+"/records", func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		var in hardwareInput
		if err := hardwareBody(e, &in); err != nil {
			return err
		}
		c, err := e.App.FindCollectionByNameOrId("hardware")
		if err != nil {
			return e.InternalServerError("Collection unavailable.", err)
		}
		r := core.NewRecord(c)
		if err := saveHardware(e.App, r, in); err != nil {
			return e.BadRequestError("Invalid hardware.", err)
		}
		return e.JSON(201, map[string]any{"id": r.Id})
	})
	s.Router.PUT(base+"/records/{id}", func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		r, err := find(e)
		if err != nil {
			return err
		}
		var in hardwareInput
		if err := hardwareBody(e, &in); err != nil {
			return err
		}
		if in.Kind != r.GetString("kind") {
			return e.BadRequestError("Hardware kind cannot change.", nil)
		}
		if err := saveHardware(e.App, r, in); err != nil {
			return e.BadRequestError("Invalid hardware.", err)
		}
		return e.JSON(200, map[string]any{"id": r.Id})
	})
	s.Router.DELETE(base+"/records/{id}", func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		r, err := find(e)
		if err != nil {
			return err
		}
		if err := e.App.Delete(r); err != nil {
			return e.InternalServerError("Unable to delete hardware.", err)
		}
		return e.JSON(200, map[string]bool{"ok": true})
	})
	s.Router.POST(base+"/records/{id}/copy", func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		r, err := find(e)
		if err != nil {
			return err
		}
		data := hardwareData(r)
		name := r.GetString("name")
		if name == "" {
			name = "未命名"
		}
		name += " 副本"
		if fields, ok := data["fields"].(map[string]any); ok {
			key := "model"
			if r.GetString("kind") == "device" {
				key = "name"
			}
			fields[key] = name
		}
		c, err := e.App.FindCollectionByNameOrId("hardware")
		if err != nil {
			return e.InternalServerError("Collection unavailable.", err)
		}
		copy := core.NewRecord(c)
		in := hardwareInput{Kind: r.GetString("kind"), Name: name, Brand: r.GetString("brand"), Subtitle: r.GetString("subtitle"), SortOrder: r.GetFloat("sort_order") + 1, Data: data}
		if err := saveHardware(e.App, copy, in); err != nil {
			return e.InternalServerError("Unable to copy hardware.", err)
		}
		return e.JSON(201, map[string]any{"id": copy.Id})
	})
	media := base + "/records/{id}/media"
	loadMedia := func(e *core.RequestEvent, id string) (*core.Record, []any, error) {
		rs, err := e.App.FindRecordsByFilter("hardware_media", "hardware = {:id}", "", 1, 0, dbx.Params{"id": id})
		if err != nil {
			return nil, nil, err
		}
		if len(rs) == 0 {
			return nil, []any{}, nil
		}
		var paths []any
		b, err := json.Marshal(rs[0].Get("paths"))
		if err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(b, &paths); err != nil {
			return nil, nil, err
		}
		if paths == nil {
			paths = []any{}
		}
		return rs[0], paths, nil
	}
	s.Router.GET(media, func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		r, err := find(e)
		if err != nil {
			return err
		}
		_, paths, err := loadMedia(e, r.Id)
		if err != nil {
			return e.InternalServerError("Unable to load media.", err)
		}
		return e.JSON(200, map[string]any{"paths": paths})
	})
	s.Router.PUT(media, func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		r, err := find(e)
		if err != nil {
			return err
		}
		var in struct {
			Paths []any `json:"paths"`
		}
		if err := hardwareBody(e, &in); err != nil {
			return err
		}
		if in.Paths == nil {
			return e.BadRequestError("Invalid media.", nil)
		}
		record, _, err := loadMedia(e, r.Id)
		if err != nil {
			return e.InternalServerError("Unable to load media.", err)
		}
		if record == nil {
			c, err := e.App.FindCollectionByNameOrId("hardware_media")
			if err != nil {
				return e.InternalServerError("Collection unavailable.", err)
			}
			record = core.NewRecord(c)
			record.Set("hardware", r.Id)
		}
		record.Set("paths", in.Paths)
		if err := e.App.Save(record); err != nil {
			return e.BadRequestError("Invalid media.", err)
		}
		return e.JSON(200, map[string]any{"paths": in.Paths})
	})
	s.Router.POST(base+"/reorder", func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		var in struct {
			IDs []string `json:"ids"`
		}
		if err := hardwareBody(e, &in); err != nil {
			return err
		}
		if in.IDs == nil {
			return e.BadRequestError("Invalid order.", nil)
		}
		err := e.App.RunInTransaction(func(tx core.App) error {
			seen := map[string]bool{}
			for i, id := range in.IDs {
				if seen[id] {
					return e.BadRequestError("Duplicate hardware.", nil)
				}
				seen[id] = true
				r, err := tx.FindRecordById("hardware", id)
				if err != nil {
					return e.BadRequestError("Hardware not found.", err)
				}
				if r.GetString("kind") != "gpu" {
					return e.BadRequestError("Invalid hardware kind.", nil)
				}
				r.Set("sort_order", i)
				if err := tx.Save(r); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return e.BadRequestError("Invalid order.", err)
		}
		return e.JSON(200, map[string]bool{"ok": true})
	})
	options := base + "/options/games"
	s.Router.GET(options, func(e *core.RequestEvent) error {
		noCache(e)
		values := map[string]bool{}
		rs, err := e.App.FindRecordsByFilter("hardware_options", "kind = 'game'", "sort_order,id", 0, 0)
		if err != nil {
			return e.InternalServerError("Unable to load options.", err)
		}
		for _, r := range rs {
			values[r.GetString("value")] = true
		}
		rs, err = e.App.FindRecordsByFilter("hardware", "", "", 0, 0)
		if err != nil {
			return e.InternalServerError("Unable to load hardware.", err)
		}
		for _, r := range rs {
			rows, _ := hardwareData(r)["rows"].(map[string]any)
			games, _ := rows["games"].([]any)
			for _, game := range games {
				row, _ := game.(map[string]any)
				name, _ := row["game"].(string)
				if name != "" {
					values[name] = true
				}
			}
		}
		out := make([]string, 0, len(values))
		for value := range values {
			if value != "" {
				out = append(out, value)
			}
		}
		sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
		return e.JSON(200, out)
	})
	s.Router.POST(options, func(e *core.RequestEvent) error {
		if err := authorize(e); err != nil {
			return err
		}
		var in struct {
			Value string `json:"value"`
		}
		if err := hardwareBody(e, &in); err != nil {
			return err
		}
		in.Value = strings.TrimSpace(in.Value)
		if in.Value == "" {
			return e.BadRequestError("Invalid option.", nil)
		}
		rs, err := e.App.FindRecordsByFilter("hardware_options", "kind = 'game' && value = {:value}", "", 1, 0, dbx.Params{"value": in.Value})
		if err != nil {
			return e.InternalServerError("Unable to load options.", err)
		}
		if len(rs) == 0 {
			c, err := e.App.FindCollectionByNameOrId("hardware_options")
			if err != nil {
				return e.InternalServerError("Collection unavailable.", err)
			}
			r := core.NewRecord(c)
			r.Set("kind", "game")
			r.Set("value", in.Value)
			if err := e.App.Save(r); err != nil {
				return e.BadRequestError("Invalid option.", err)
			}
		}
		return e.JSON(200, map[string]bool{"ok": true})
	})
}
