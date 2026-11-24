package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func serveGPU(app core.App, w http.ResponseWriter, r *http.Request, canSeePrivate bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	var result any
	var err error
	defer func() {
		if err != nil {
			status = http.StatusInternalServerError
			result = map[string]any{"error": err.Error()}
		}
		if !canSeePrivate && r.Method == "GET" && status < 400 {
			if rows, ok := result.([]gpuRow); ok {
				for _, row := range rows {
					stripRelatedMedia(row)
				}
			} else {
				stripRelatedMedia(result)
			}
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	}()
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	path = strings.TrimPrefix(path, "hpahub/gpu/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	method := r.Method
	body := gpuRow{}
	if method == "POST" || method == "PUT" {
		data, e := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024+1))
		if e != nil {
			err = e
			return
		}
		if len(data) > 8*1024*1024 {
			status = 413
			result = gpuRow{"error": "请求体过大"}
			return
		}
		if len(data) > 0 {
			if e = json.Unmarshal(data, &body); e != nil || body == nil {
				status = 400
				result = gpuRow{"error": "JSON 解析失败"}
				return
			}
		}
	}
	ok := gpuRow{"ok": true}
	if len(parts) == 1 && parts[0] == "stats" && method == "GET" {
		result, err = gpuStats(app)
		return
	}
	if len(parts) == 1 && parts[0] == "related-media" && (method == "GET" || method == "POST") {
		id, _ := strconv.ParseInt(r.URL.Query().Get("gpu_id"), 10, 64)
		media := []any{}
		work := func(tx core.App) error {
			rows, e := gpuQuery(tx, "SELECT related_media FROM gpus WHERE id={:id}", dbx.Params{"id": id})
			if e != nil {
				return e
			}
			if len(rows) > 0 {
				if e = json.Unmarshal([]byte(gpuString(rows[0]["related_media"])), &media); e != nil {
					return e
				}
			}
			if method != "POST" {
				return nil
			}
			path, valid := body["path"].(string)
			action, _ := body["action"].(string)
			if valid && action == "add" && path != "" {
				found := false
				for _, p := range media {
					if s, ok := p.(string); ok && s == path {
						found = true
					}
				}
				if !found {
					media = append(media, path)
				}
			} else if valid && action == "remove" {
				out := []any{}
				for _, p := range media {
					if s, ok := p.(string); !ok || s != path {
						out = append(out, p)
					}
				}
				media = out
			}
			b, e := json.Marshal(media)
			if e != nil {
				return e
			}
			return gpuExec(tx, "UPDATE gpus SET related_media={:media} WHERE id={:id}", dbx.Params{"id": id, "media": string(b)})
		}
		if method == "POST" {
			err = app.RunInTransaction(work)
		} else {
			err = work(app)
		}
		result = gpuRow{"ok": true, "media": media}
		return
	}
	if len(parts) == 2 && parts[0] == "options" && parts[1] == "games" {
		if method == "GET" {
			var rows []gpuRow
			rows, err = gpuQuery(app, "SELECT value FROM (SELECT value FROM option_values WHERE kind='game' UNION SELECT game AS value FROM games WHERE game IS NOT NULL AND game <> '') ORDER BY value COLLATE NOCASE", nil)
			values := []any{}
			for _, row := range rows {
				values = append(values, row["value"])
			}
			result = values
			return
		}
		if method == "POST" {
			v := strings.TrimSpace(gpuString(body["value"]))
			if v != "" {
				err = gpuExec(app, "INSERT OR IGNORE INTO option_values (kind,value) VALUES ('game',{:value})", dbx.Params{"value": v})
			}
			result = ok
			return
		}
	}
	if len(parts) == 1 && parts[0] == "reorder" && method == "POST" {
		ids, valid := body["ids"].([]any)
		if !valid {
			status = 400
			result = gpuRow{"error": "缺少 ids"}
			return
		}
		err = app.RunInTransaction(func(tx core.App) error {
			for i, id := range ids {
				if e := gpuExec(tx, "UPDATE gpus SET sort_order={:ord} WHERE id={:id}", dbx.Params{"ord": i, "id": id}); e != nil {
					return e
				}
			}
			return nil
		})
		result = ok
		return
	}
	if len(parts) > 0 && (parts[0] == "gpus" || parts[0] == "items") {
		table := parts[0]
		if len(parts) == 1 {
			if method == "GET" {
				if table == "gpus" {
					result, err = gpuQuery(app, `SELECT g.*, (SELECT COUNT(*) FROM games WHERE gpu_id=g.id) AS game_count, (SELECT COUNT(*) FROM productivity WHERE gpu_id=g.id) AS prod_count, (SELECT COUNT(*) FROM codec_runs WHERE gpu_id=g.id AND kind='encoding') AS enc_count, (SELECT COUNT(*) FROM codec_runs WHERE gpu_id=g.id AND kind='decoding') AS dec_count, (SELECT steel_nomad_light FROM mark3d WHERE gpu_id=g.id) AS mark3d_light, (SELECT time_spy FROM mark3d WHERE gpu_id=g.id) AS time_spy, (SELECT solar_bay FROM mark3d WHERE gpu_id=g.id) AS solar_bay FROM gpus g ORDER BY g.sort_order,g.id`, nil)
				} else {
					q := "SELECT * FROM items"
					params := dbx.Params{}
					if kind := r.URL.Query().Get("kind"); kind != "" {
						q += " WHERE kind={:kind}"
						params["kind"] = kind
					}
					q += " ORDER BY kind,sort_order,id"
					var rows []gpuRow
					rows, err = gpuQuery(app, q, params)
					for _, row := range rows {
						gpuItemParse(row)
					}
					result = rows
				}
				return
			}
			if method == "POST" {
				var id int64
				if table == "gpus" {
					id, err = gpuSave(app, 0, body)
				} else {
					if gpuString(body["kind"]) == "" {
						status = 400
						result = gpuRow{"error": "缺少 kind"}
						return
					}
					id, err = gpuItemSave(app, 0, body)
				}
				status = 201
				result = gpuRow{"id": id}
				return
			}
		} else {
			id, e := strconv.ParseInt(parts[1], 10, 64)
			if e != nil || id <= 0 {
				status = 400
				result = gpuRow{"error": "id 非法"}
				return
			}
			if len(parts) == 2 {
				if method == "GET" {
					var row gpuRow
					if table == "gpus" {
						row, err = gpuGet(app, id)
					} else {
						var rows []gpuRow
						rows, err = gpuQuery(app, "SELECT * FROM items WHERE id={:id}", dbx.Params{"id": id})
						if len(rows) > 0 {
							row = gpuItemParse(rows[0])
						}
					}
					if err != nil {
						return
					}
					if row == nil {
						status = 404
						result = gpuRow{"error": "未找到"}
					} else {
						result = row
					}
					return
				}
				if method == "PUT" {
					var existing []gpuRow
					existing, err = gpuQuery(app, "SELECT id FROM "+table+" WHERE id={:id}", dbx.Params{"id": id})
					if err != nil {
						return
					}
					if len(existing) == 0 {
						status = 404
						result = gpuRow{"error": "未找到"}
						return
					}
					if table == "gpus" {
						_, err = gpuSave(app, id, body)
					} else {
						_, err = gpuItemSave(app, id, body)
					}
					result = gpuRow{"ok": true, "id": id}
					return
				}
				if method == "DELETE" {
					err = app.RunInTransaction(func(tx core.App) error {
						if table == "gpus" {
							for _, t := range []string{"cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "gpu_custom"} {
								if e := gpuExec(tx, "DELETE FROM "+t+" WHERE gpu_id={:id}", dbx.Params{"id": id}); e != nil {
									return e
								}
							}
						}
						return gpuExec(tx, "DELETE FROM "+table+" WHERE id={:id}", dbx.Params{"id": id})
					})
					result = ok
					return
				}
			} else if len(parts) == 3 && parts[2] == "copy" && method == "POST" {
				var newID int64
				if table == "gpus" {
					newID, err = gpuCopy(app, id)
				} else {
					var rows []gpuRow
					rows, err = gpuQuery(app, "SELECT * FROM items WHERE id={:id}", dbx.Params{"id": id})
					if err == nil && len(rows) > 0 {
						row := gpuItemParse(rows[0])
						name := gpuString(row["name"])
						if name == "" {
							name = "未命名"
						}
						row["name"] = name + " 副本"
						order, _ := strconv.ParseInt(gpuString(row["sort_order"]), 10, 64)
						row["sort_order"] = order + 1
						newID, err = gpuItemSave(app, 0, row)
					}
				}
				if err != nil {
					return
				}
				if newID == 0 {
					status = 404
					result = gpuRow{"error": "未找到"}
				} else {
					status = 201
					result = gpuRow{"id": newID}
				}
				return
			}
		}
	}
	status = 404
	result = gpuRow{"error": "接口不存在: " + r.URL.Path}
}

func gpuStats(app core.App) (gpuRow, error) {
	out := gpuRow{}
	for k, q := range map[string]string{"gpus": "SELECT count(*) n FROM gpus", "games": "SELECT count(*) n FROM games", "productivity": "SELECT count(*) n FROM productivity", "encoding": "SELECT count(*) n FROM codec_runs WHERE kind='encoding'", "decoding": "SELECT count(*) n FROM codec_runs WHERE kind='decoding'", "withMark3d": "SELECT count(*) n FROM mark3d"} {
		rows, e := gpuQuery(app, q, nil)
		if e != nil {
			return nil, e
		}
		out[k] = rows[0]["n"]
	}
	rows, e := gpuQuery(app, "SELECT kind,count(*) n FROM items GROUP BY kind", nil)
	if e != nil {
		return nil, e
	}
	kinds := gpuRow{}
	for _, row := range rows {
		kinds[fmt.Sprint(row["kind"])] = row["n"]
	}
	out["itemsByKind"] = kinds
	return out, nil
}
