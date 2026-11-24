package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func TestImportGPU(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gpu.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		PRAGMA user_version=1;
		CREATE TABLE gpus (id INTEGER PRIMARY KEY, model TEXT NOT NULL DEFAULT '', brand TEXT NOT NULL DEFAULT '', vram_size TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0, related_media TEXT NOT NULL DEFAULT '[]');
		CREATE TABLE cooling (gpu_id INTEGER PRIMARY KEY, core_temp TEXT NOT NULL DEFAULT '');
		CREATE TABLE mark3d (gpu_id INTEGER PRIMARY KEY, time_spy TEXT NOT NULL DEFAULT '');
		CREATE TABLE blender (gpu_id INTEGER PRIMARY KEY, monster TEXT NOT NULL DEFAULT '');
		CREATE TABLE sw (gpu_id INTEGER PRIMARY KEY, graphics TEXT NOT NULL DEFAULT '');
		CREATE TABLE productivity (id INTEGER PRIMARY KEY, gpu_id INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE games (id INTEGER PRIMARY KEY, gpu_id INTEGER NOT NULL, game TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE codec_runs (id INTEGER PRIMARY KEY, gpu_id INTEGER NOT NULL, kind TEXT NOT NULL, codec TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE items (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', brand TEXT NOT NULL DEFAULT '', subtitle TEXT NOT NULL DEFAULT '', fields TEXT NOT NULL DEFAULT '{}', rows TEXT NOT NULL DEFAULT '{}', sort_order INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE gpu_custom (gpu_id INTEGER NOT NULL, section TEXT NOT NULL, field_key TEXT NOT NULL, label TEXT NOT NULL DEFAULT '', value TEXT NOT NULL DEFAULT '', hidden INTEGER NOT NULL DEFAULT 0, sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (gpu_id, section, field_key));
		CREATE TABLE option_values (kind TEXT NOT NULL, value TEXT NOT NULL, sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (kind, value));
		INSERT INTO gpus VALUES (7, 'GPU', 'Brand', '16', 4, '["private/media"]');
		INSERT INTO cooling VALUES (7, '70');
		INSERT INTO mark3d VALUES (7, '12000');
		INSERT INTO blender VALUES (7, '3');
		INSERT INTO sw VALUES (7, '4');
		INSERT INTO productivity VALUES (8, 7, 'app', 1);
		INSERT INTO games VALUES (9, 7, 'game', 2);
		INSERT INTO codec_runs VALUES (10, 7, 'encoding', 'h264', 1);
		INSERT INTO codec_runs VALUES (11, 7, 'decoding', 'av1', 2);
		INSERT INTO items VALUES (12, 'cpu', 'CPU', 'Brand', 'subtitle', '{"cores":8}', '{"bench":[{"score":42}]}', 3);
		INSERT INTO items VALUES (13, 'device', 'Device', '', '', '{"color":"black"}', '{"ports":[]}', 5);
		INSERT INTO gpu_custom VALUES (7, 'basic', 'key', 'Label', 'value', 0, 1);
		INSERT INTO option_values VALUES ('game', 'option', 6);
	`)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("hardware")
	if err != nil {
		t.Fatal(err)
	}
	conflict := core.NewRecord(collection)
	conflict.Set("kind", "gpu")
	conflict.Set("name", "existing")
	if err := app.Save(conflict); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil {
		t.Fatal("nonempty target accepted")
	}
	if err := app.Delete(conflict); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("nonempty target accepted on second import: %v", err)
	}
	var sourceName, sourceMedia string
	if err := db.QueryRow("SELECT model, related_media FROM gpus WHERE id=7").Scan(&sourceName, &sourceMedia); err != nil || sourceName != "GPU" || sourceMedia != `["private/media"]` {
		t.Fatalf("source changed: name=%q media=%q err=%v", sourceName, sourceMedia, err)
	}
	records, err := app.FindRecordsByFilter("hardware", "", "", 0, 0)
	if err != nil || len(records) != 3 {
		t.Fatalf("hardware count=%d err=%v", len(records), err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindHardwareRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hpahub/hardware/records", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"GPU"`) || !strings.Contains(response.Body.String(), `"vram_size":"16"`) || strings.Contains(response.Body.String(), "private/media") {
		t.Fatalf("imported hardware display: %d %s", response.Code, response.Body.String())
	}
	gpu, err := app.FindFirstRecordByFilter("hardware", "kind = 'gpu' && name = 'GPU'")
	if err != nil {
		t.Fatal(err)
	}
	jsonText := func(value any) string {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	if gpu.GetString("brand") != "Brand" || gpu.GetFloat("sort_order") != 4 {
		t.Fatalf("GPU metadata: %#v", hardwareView(gpu))
	}
	data := hardwareData(gpu)
	if value := jsonText(data); strings.Contains(value, `"gpu_id"`) || strings.Contains(value, `"legacy"`) {
		t.Fatalf("legacy storage identifiers retained: %s", value)
	}
	fields, ok := data["fields"].(map[string]any)
	if !ok || fields["vram_size"] != "16" || fields["model"] != "GPU" {
		t.Fatalf("GPU fields: %#v", data)
	}
	if _, ok := fields["related_media"]; ok {
		t.Fatal("private media stored in public fields")
	}
	sections, ok := data["sections"].(map[string]any)
	if !ok {
		t.Fatalf("sections: %#v", data)
	}
	for key, field := range map[string]string{"cooling": "core_temp", "mark3d": "time_spy", "blender": "monster", "sw": "graphics"} {
		section, ok := sections[key].(map[string]any)
		if !ok || section[field] == "" || section[field] == nil {
			t.Fatalf("section %s: %#v", key, sections[key])
		}
	}
	rows, ok := data["rows"].(map[string]any)
	if !ok {
		t.Fatalf("rows: %#v", data)
	}
	for _, key := range []string{"productivity", "games", "encoding", "decoding"} {
		items, ok := rows[key].([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("rows %s: %#v", key, rows[key])
		}
	}
	if data["custom"] == nil || !strings.Contains(jsonText(data["custom"]), "value") {
		t.Fatalf("custom: %#v", data["custom"])
	}
	media, err := app.FindFirstRecordByFilter("hardware_media", "hardware = {:id}", dbx.Params{"id": gpu.Id})
	if err != nil || !strings.Contains(jsonText(media.Get("paths")), "private/media") {
		t.Fatalf("media: %v", err)
	}
	cpu, err := app.FindFirstRecordByFilter("hardware", "kind = 'cpu' && name = 'CPU'")
	if err != nil || !strings.Contains(jsonText(hardwareData(cpu)), `"cores":8`) || !strings.Contains(jsonText(hardwareData(cpu)), `"score":42`) {
		t.Fatalf("CPU import: %v", err)
	}
	device, err := app.FindFirstRecordByFilter("hardware", "kind = 'device' && name = 'Device'")
	if err != nil || !strings.Contains(jsonText(hardwareData(device)), `"color":"black"`) || !strings.Contains(jsonText(hardwareData(device)), `"ports":[]`) {
		t.Fatalf("device import: %v", err)
	}
	option, err := app.FindFirstRecordByFilter("hardware_options", "kind = 'game' && value = 'option'")
	if err != nil || option.GetFloat("sort_order") != 6 {
		t.Fatalf("option import: %v", err)
	}
	records, err = app.FindRecordsByFilter("hardware", "", "", 0, 0)
	if err != nil || len(records) != 3 {
		t.Fatalf("conflict changed target: count=%d err=%v", len(records), err)
	}
	if _, err := db.Exec("PRAGMA user_version=2"); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil {
		t.Fatal("unsupported version accepted")
	}
}

func TestImportGPUOldSchema(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE gpus (id INTEGER PRIMARY KEY, model TEXT NOT NULL DEFAULT ''); INSERT INTO gpus VALUES (5, 'old')"); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindFirstRecordByFilter("hardware", "kind = 'gpu' && name = 'old'"); err != nil {
		t.Fatal(err)
	}
}

func TestImportGPURollbackAndReadOnly(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	if err := importGPU(app, missing); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing source created or inaccessible: %v", err)
	}
	path := filepath.Join(t.TempDir(), "rollback.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE gpus (id INTEGER PRIMARY KEY, model TEXT NOT NULL DEFAULT '');
		CREATE TABLE items (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', fields TEXT NOT NULL DEFAULT '{}', rows TEXT NOT NULL DEFAULT '{}');
		INSERT INTO gpus VALUES (1, 'GPU');
		INSERT INTO items VALUES (2, 'invalid', 'CPU', '{}', '{}');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil {
		t.Fatal("invalid item accepted")
	}
	for _, table := range []string{"hardware", "hardware_media", "hardware_options"} {
		var count int
		if err := app.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil || count != 0 {
			t.Fatalf("rollback %s: count=%d err=%v", table, count, err)
		}
	}
	if _, err := db.Exec("UPDATE items SET kind='cpu'"); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
	records, err := app.FindRecordsByFilter("hardware", "", "", 0, 0)
	if err != nil || len(records) != 2 {
		t.Fatalf("retry records=%d err=%v", len(records), err)
	}
}
