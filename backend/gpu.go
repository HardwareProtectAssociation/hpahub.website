package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type gpuRow = map[string]any

var gpuColumns = strings.Fields("model brand vram_size vram_type vram_clock gpu_clock tgp actual_power driver_version test_date test_platform ambient_temp note sort_order")
var gpuChildColumns = map[string][]string{
	"cooling": strings.Fields("default_fan_temp max_fan_temp temp_45db core_temp hotspot_temp vram_temp default_fan_rpm max_fan_rpm rpm_45db full_load_power power_45db temp_limit power_limit throttling"),
	"mark3d":  strings.Fields("solar_bay steel_nomad steel_nomad_light speed_way port_royal time_spy fire_strike dx_raytracing pcie_bandwidth"),
	"blender": strings.Fields("monster junkshop classroom"), "sw": strings.Fields("graphics processor io realview simulation"),
	"productivity": strings.Fields("name score unit note"),
	"games":        strings.Fields("game resolution quality ray_tracing render_mode upscale_tech upscale_quality frame_gen avg_fps min_fps one_percent_low gpu_temp gpu_usage cpu_usage ram_usage vram_usage power note"),
	"codec_runs":   strings.Fields("codec resolution time_sec power gpu_usage cpu_usage note"),
}

func gpuQuery(app core.App, query string, params dbx.Params) ([]gpuRow, error) {
	rows, err := app.DB().NewQuery(query).Bind(params).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []gpuRow{}
	for rows.Next() {
		values := make([]any, len(cols))
		refs := make([]any, len(cols))
		for i := range values {
			refs[i] = &values[i]
		}
		if err := rows.Scan(refs...); err != nil {
			return nil, err
		}
		row := gpuRow{}
		for i, c := range cols {
			if b, ok := values[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = values[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func gpuExec(app core.App, query string, params dbx.Params) error {
	_, err := app.DB().NewQuery(query).Bind(params).Execute()
	return err
}
func gpuInsert(app core.App, table string, row gpuRow) (int64, error) {
	cols := make([]string, 0, len(row))
	vals := make([]string, 0, len(row))
	params := dbx.Params{}
	for k, v := range row {
		cols = append(cols, k)
		vals = append(vals, "{:"+k+"}")
		params[k] = v
	}
	result, err := app.DB().NewQuery("INSERT INTO " + table + " (" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(vals, ",") + ")").Bind(params).Execute()
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
func gpuObject(v any) gpuRow {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return gpuRow{}
}
func gpuString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		return "[object Object]"
	case []any:
		a := make([]string, len(x))
		for i, v := range x {
			a[i] = gpuString(v)
		}
		return strings.Join(a, ",")
	default:
		return fmt.Sprint(v)
	}
}
func gpuPick(v any, cols []string) gpuRow {
	m := gpuObject(v)
	out := gpuRow{}
	for _, c := range cols {
		out[c] = gpuString(m[c])
	}
	return out
}
func gpuNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
func gpuRows(app core.App, table string, id int64, suffix string) ([]gpuRow, error) {
	return gpuQuery(app, "SELECT * FROM "+table+" WHERE gpu_id={:id} "+suffix, dbx.Params{"id": id})
}
func gpuGet(app core.App, id int64) (gpuRow, error) {
	rows, err := gpuQuery(app, "SELECT * FROM gpus WHERE id={:id}", dbx.Params{"id": id})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	out := gpuRow{"gpu": rows[0]}
	for _, table := range []string{"cooling", "mark3d", "blender", "sw"} {
		r, e := gpuRows(app, table, id, "")
		if e != nil {
			return nil, e
		}
		out[table] = nil
		if len(r) > 0 {
			out[table] = r[0]
		}
	}
	for _, pair := range [][2]string{{"productivity", "productivity"}, {"games", "games"}, {"codec_runs", "encoding"}, {"codec_runs", "decoding"}} {
		suffix := "ORDER BY sort_order,id"
		if pair[0] == "codec_runs" {
			suffix = "AND kind='" + pair[1] + "' " + suffix
		}
		r, e := gpuRows(app, pair[0], id, suffix)
		if e != nil {
			return nil, e
		}
		out[pair[1]] = r
	}
	custom := gpuRow{}
	for _, s := range []string{"basic", "cooling", "mark3d"} {
		custom[s] = gpuRow{"hidden": []any{}, "extra": []any{}}
	}
	r, e := gpuRows(app, "gpu_custom", id, "ORDER BY section,sort_order,rowid")
	if e != nil {
		return nil, e
	}
	for _, v := range r {
		s, ok := custom[gpuString(v["section"])].(gpuRow)
		if !ok {
			continue
		}
		if v["hidden"] != int64(0) {
			s["hidden"] = append(s["hidden"].([]any), v["field_key"])
		} else {
			s["extra"] = append(s["extra"].([]any), gpuRow{"key": v["field_key"], "label": v["label"], "value": v["value"]})
		}
	}
	out["custom"] = custom
	return out, nil
}
func gpuChildren(app core.App, id int64, p gpuRow) error {
	for _, t := range []string{"cooling", "mark3d", "blender", "sw"} {
		if p[t] != nil {
			row := gpuPick(p[t], gpuChildColumns[t])
			row["gpu_id"] = id
			if _, err := gpuInsert(app, t, row); err != nil {
				return err
			}
		}
	}
	for _, pair := range [][2]string{{"productivity", "productivity"}, {"games", "games"}, {"codec_runs", "encoding"}, {"codec_runs", "decoding"}} {
		if p[pair[1]] == nil {
			continue
		}
		list, ok := p[pair[1]].([]any)
		if !ok {
			return fmt.Errorf("invalid %s", pair[1])
		}
		for i, v := range list {
			row := gpuPick(v, gpuChildColumns[pair[0]])
			row["gpu_id"] = id
			row["sort_order"] = i
			if pair[0] == "codec_runs" {
				row["kind"] = pair[1]
			}
			if _, err := gpuInsert(app, pair[0], row); err != nil {
				return err
			}
		}
	}
	custom := gpuObject(p["custom"])
	for _, section := range []string{"basic", "cooling", "mark3d"} {
		s := gpuObject(custom[section])
		n := 0
		for _, key := range []string{"hidden", "extra"} {
			list, _ := s[key].([]any)
			for _, v := range list {
				row := gpuRow{"gpu_id": id, "section": section, "field_key": "", "label": "", "value": "", "hidden": 0, "sort_order": n}
				if key == "hidden" {
					row["field_key"] = gpuString(v)
					row["hidden"] = 1
				} else {
					e := gpuObject(v)
					row["field_key"] = gpuString(e["key"])
					row["label"] = gpuString(e["label"])
					row["value"] = gpuString(e["value"])
				}
				if row["field_key"] == "" {
					continue
				}
				if _, err := gpuInsert(app, "gpu_custom", row); err != nil {
					return err
				}
				n++
			}
		}
	}
	return nil
}
func gpuSave(app core.App, id int64, p gpuRow) (int64, error) {
	err := app.RunInTransaction(func(tx core.App) error {
		row := gpuPick(p["gpu"], gpuColumns)
		row["updated_at"] = gpuNow()
		if id == 0 {
			row["created_at"] = row["updated_at"]
			var e error
			id, e = gpuInsert(tx, "gpus", row)
			if e != nil {
				return e
			}
		} else {
			sets := []string{}
			params := dbx.Params{"id": id}
			for k, v := range row {
				sets = append(sets, k+"={:"+k+"}")
				params[k] = v
			}
			if err := gpuExec(tx, "UPDATE gpus SET "+strings.Join(sets, ",")+" WHERE id={:id}", params); err != nil {
				return err
			}
			for _, t := range []string{"cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "gpu_custom"} {
				if err := gpuExec(tx, "DELETE FROM "+t+" WHERE gpu_id={:id}", dbx.Params{"id": id}); err != nil {
					return err
				}
			}
		}
		return gpuChildren(tx, id, p)
	})
	return id, err
}
func gpuCopy(app core.App, id int64) (int64, error) {
	var newID int64
	err := app.RunInTransaction(func(tx core.App) error {
		src, e := gpuGet(tx, id)
		if e != nil || src == nil {
			return e
		}
		g := src["gpu"].(gpuRow)
		delete(g, "id")
		name := gpuString(g["model"])
		if name == "" {
			name = "未命名显卡"
		}
		g["model"] = name + " 副本"
		order, _ := strconv.ParseInt(gpuString(g["sort_order"]), 10, 64)
		g["sort_order"] = order + 1
		newID, e = gpuInsert(tx, "gpus", g)
		if e != nil {
			return e
		}
		for _, t := range []string{"cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "gpu_custom"} {
			rows, e := gpuRows(tx, t, id, "")
			if e != nil {
				return e
			}
			for _, r := range rows {
				delete(r, "id")
				r["gpu_id"] = newID
				if _, e = gpuInsert(tx, t, r); e != nil {
					return e
				}
			}
		}
		return nil
	})
	return newID, err
}
func gpuItemParse(row gpuRow) gpuRow {
	for _, k := range []string{"fields", "rows"} {
		var v any
		if json.Unmarshal([]byte(gpuString(row[k])), &v) != nil {
			v = gpuRow{}
		}
		row[k] = v
	}
	return row
}
func gpuItemSave(app core.App, id int64, p gpuRow) (int64, error) {
	row := gpuRow{}
	if id != 0 {
		r, e := gpuQuery(app, "SELECT * FROM items WHERE id={:id}", dbx.Params{"id": id})
		if e != nil {
			return 0, e
		}
		if len(r) == 0 {
			return 0, nil
		}
		row = gpuItemParse(r[0])
	}
	for _, k := range []string{"kind", "name", "brand", "subtitle", "fields", "rows", "sort_order"} {
		if p[k] != nil {
			row[k] = p[k]
		} else if row[k] == nil {
			if k == "fields" || k == "rows" {
				row[k] = gpuRow{}
			} else if k == "sort_order" {
				row[k] = 0
			} else {
				row[k] = ""
			}
		}
	}
	for _, k := range []string{"kind", "name", "brand", "subtitle"} {
		row[k] = gpuString(row[k])
	}
	for _, k := range []string{"fields", "rows"} {
		b, e := json.Marshal(row[k])
		if e != nil {
			return 0, e
		}
		row[k] = string(b)
	}
	row["updated_at"] = gpuNow()
	if id == 0 {
		row["created_at"] = row["updated_at"]
		return gpuInsert(app, "items", row)
	}
	delete(row, "id")
	delete(row, "created_at")
	sets := []string{}
	params := dbx.Params{"id": id}
	for k, v := range row {
		sets = append(sets, k+"={:"+k+"}")
		params[k] = v
	}
	return id, gpuExec(app, "UPDATE items SET "+strings.Join(sets, ",")+" WHERE id={:id}", params)
}
