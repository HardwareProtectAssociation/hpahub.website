package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestImportGPU(t *testing.T) {
	app := invitationTestApp(t)
	path := filepath.Join(t.TempDir(), "gpu.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := `
		CREATE TABLE gpus (id INTEGER PRIMARY KEY AUTOINCREMENT, model TEXT NOT NULL DEFAULT '', related_media TEXT NOT NULL DEFAULT '[]');
		CREATE TABLE cooling (gpu_id INTEGER PRIMARY KEY, core_temp TEXT NOT NULL DEFAULT '');
		CREATE TABLE mark3d (gpu_id INTEGER PRIMARY KEY, time_spy TEXT NOT NULL DEFAULT '');
		CREATE TABLE blender (gpu_id INTEGER PRIMARY KEY, monster TEXT NOT NULL DEFAULT '');
		CREATE TABLE sw (gpu_id INTEGER PRIMARY KEY, graphics TEXT NOT NULL DEFAULT '');
		CREATE TABLE productivity (id INTEGER PRIMARY KEY AUTOINCREMENT, gpu_id INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '');
		CREATE TABLE games (id INTEGER PRIMARY KEY AUTOINCREMENT, gpu_id INTEGER NOT NULL, game TEXT NOT NULL DEFAULT '');
		CREATE TABLE codec_runs (id INTEGER PRIMARY KEY AUTOINCREMENT, gpu_id INTEGER NOT NULL, kind TEXT NOT NULL);
		CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, fields TEXT NOT NULL DEFAULT '{}');
		CREATE TABLE gpu_custom (gpu_id INTEGER NOT NULL, section TEXT NOT NULL, field_key TEXT NOT NULL, value TEXT NOT NULL DEFAULT '', PRIMARY KEY (gpu_id, section, field_key));
		CREATE TABLE option_values (kind TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY (kind, value));`
	if _, err := db.Exec(schema + `
		PRAGMA user_version = 1;
		INSERT INTO gpus (id, model, related_media) VALUES (7, 'GPU', '["media"]');
		INSERT INTO cooling VALUES (7, '70'); INSERT INTO mark3d VALUES (7, '12000');
		INSERT INTO blender VALUES (7, '3'); INSERT INTO sw VALUES (7, '4');
		INSERT INTO productivity VALUES (8, 7, 'app'); INSERT INTO games VALUES (9, 7, 'game');
		INSERT INTO codec_runs VALUES (10, 7, 'encode'); INSERT INTO items VALUES (11, 'cpu', '{"a":1}');
		INSERT INTO gpu_custom VALUES (7, 'basic', 'key', 'value'); INSERT INTO option_values VALUES ('game', 'option');
		UPDATE sqlite_sequence SET seq = 100 WHERE name = 'gpus';`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB().NewQuery("INSERT INTO gpus (id, model) VALUES (7, 'conflict')").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil {
		t.Fatal("conflicting import accepted")
	}
	var markers int
	if err := app.DB().NewQuery("SELECT count(*) FROM hpahub_gpu_import WHERE id = 1").Row(&markers); err != nil || markers != 0 {
		t.Fatalf("failed import marked complete: count=%d err=%v", markers, err)
	}
	if _, err := app.DB().NewQuery("DELETE FROM gpus WHERE id = 7").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
	if err := app.DB().NewQuery("SELECT count(*) FROM hpahub_gpu_import WHERE id = 1").Row(&markers); err != nil || markers != 1 {
		t.Fatalf("completed import marker: count=%d err=%v", markers, err)
	}
	for _, table := range []string{"gpus", "cooling", "mark3d", "blender", "sw", "productivity", "games", "codec_runs", "items", "gpu_custom", "option_values"} {
		var count int
		if err := app.DB().NewQuery("SELECT count(*) FROM " + table).Row(&count); err != nil || count != 1 {
			t.Fatalf("%s: count=%d err=%v", table, count, err)
		}
	}
	var media string
	if err := app.DB().NewQuery("SELECT related_media FROM gpus WHERE id=7").Row(&media); err != nil || media != `["media"]` {
		t.Fatalf("media=%q err=%v", media, err)
	}
	var seq int
	if err := app.DB().NewQuery("SELECT seq FROM sqlite_sequence WHERE name='gpus'").Row(&seq); err != nil || seq != 100 {
		t.Fatalf("seq=%d err=%v", seq, err)
	}
	if _, err := db.Exec("INSERT INTO gpus (id, model) VALUES (12, 'new'); UPDATE games SET game='changed'"); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err == nil {
		t.Fatal("conflicting row accepted")
	}
	var count int
	if err := app.DB().NewQuery("SELECT count(*) FROM gpus WHERE id=12").Row(&count); err != nil || count != 0 {
		t.Fatalf("partial import: count=%d err=%v", count, err)
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
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE gpus (id INTEGER PRIMARY KEY AUTOINCREMENT, model TEXT NOT NULL DEFAULT ''); INSERT INTO gpus VALUES (5, 'old')"); err != nil {
		t.Fatal(err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
	var media string
	if err := app.DB().NewQuery("SELECT related_media FROM gpus WHERE id=5").Row(&media); err != nil || media != "[]" {
		t.Fatalf("media=%q err=%v", media, err)
	}
	if err := importGPU(app, path); err != nil {
		t.Fatal(err)
	}
}
