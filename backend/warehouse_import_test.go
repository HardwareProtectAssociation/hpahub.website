package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func warehouseTestExtractBackup(t *testing.T, backupPath, restoredDir string) {
	t.Helper()
	backup, err := zip.OpenReader(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	for _, entry := range backup.File {
		name := filepath.FromSlash(entry.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			t.Fatalf("invalid backup path %q", entry.Name)
		}
		target := filepath.Join(restoredDir, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		in, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			t.Fatal(err)
		}
		_, err = io.Copy(out, in)
		closeOut := out.Close()
		closeIn := in.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeOut != nil {
			t.Fatal(closeOut)
		}
		if closeIn != nil {
			t.Fatal(closeIn)
		}
	}
}

func TestWarehouseImportBackupRestore(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	source := t.TempDir()
	for _, dir := range []string{"projects/empty", "projects/demo/videos/.thumbs", "projects/demo/videos/.web", "projects/demo/videos/.preview", "projects/demo/docs"} {
		if err := os.MkdirAll(filepath.Join(source, filepath.FromSlash(dir)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"storage_config.json":                        `{"allocations":{"docs":12}}`,
		"projects/demo/.meta.json":                   `{"display_name":"demo","extra":true}`,
		"projects/demo/videos/movie.mp4":             "video-data",
		"projects/demo/videos/.thumbs/movie.mp4.jpg": "thumbnail-data",
		"projects/demo/videos/.web/movie.mp4":        "web-data",
		"projects/demo/videos/.preview/movie.mp4":    "preview-data",
		"projects/demo/docs/empty.txt":               "",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(source, filepath.FromSlash(name)), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := importWarehouse(app, source); err != nil {
		t.Fatal(err)
	}
	if err := importWarehouse(app, source); err != nil {
		t.Fatalf("repeat import: %v", err)
	}
	if err := app.CreateBackup(context.Background(), "warehouse-test.zip"); err != nil {
		t.Fatal(err)
	}

	restoredDir := t.TempDir()
	warehouseTestExtractBackup(t, filepath.Join(app.DataDir(), "backups", "warehouse-test.zip"), restoredDir)
	restored := core.NewBaseApp(core.BaseAppConfig{DataDir: restoredDir})
	if err := restored.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer restored.ResetBootstrapState()
	storage, err := restored.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	project, err := restored.FindFirstRecordByFilter("warehouse_projects", "name = 'demo'")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	encoded, err := json.Marshal(project.Get("metadata"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &metadata); err != nil || metadata["display_name"] != "demo" || metadata["extra"] != true {
		t.Fatalf("restored metadata: %s (%v)", encoded, err)
	}
	if _, err := restored.FindFirstRecordByFilter("warehouse_projects", "name = 'empty'"); err == nil {
		t.Fatal("empty legacy directory became a project")
	}
	settings, err := restored.FindFirstRecordByFilter("warehouse_settings", "key = 'storage_config'")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(settings.Get("value"))
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Allocations map[string]int `json:"allocations"`
	}
	if err := json.Unmarshal(encoded, &value); err != nil || value.Allocations["docs"] != 12 {
		t.Fatalf("restored settings: %s (%v)", encoded, err)
	}
	video, err := restored.FindFirstRecordByFilter("warehouse_files", "project = {:project} && category = 'videos' && name = 'movie.mp4'", dbx.Params{"project": project.Id})
	if err != nil {
		t.Fatal(err)
	}
	document, err := restored.FindFirstRecordByFilter("warehouse_files", "project = {:project} && category = 'docs' && name = 'empty.txt'", dbx.Params{"project": project.Id})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		record   *core.Record
		field    string
		expected string
	}{
		{video, "content", "video-data"},
		{video, "thumbnail", "thumbnail-data"},
		{video, "web", "web-data"},
		{video, "preview", "preview-data"},
		{document, "content", ""},
	} {
		name := item.record.GetString(item.field)
		if name == "" {
			t.Fatalf("missing restored %s", item.field)
		}
		reader, err := storage.GetReader(item.record.BaseFilesPath() + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != item.expected {
			t.Fatalf("restored %s: %q", item.field, data)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "projects", "demo", "docs", "empty.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := importWarehouse(app, source); err == nil {
		t.Fatal("changed source did not conflict")
	}
}

func TestWarehouseImportRejectsSymlinksAndCanRetry(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	source := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret")
	if err := os.WriteFile(target, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(source, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := importWarehouse(app, source); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := app.FindFirstRecordByFilter("warehouse_settings", "key = '__import_manifest'"); err == nil {
		t.Fatal("failed import marked complete")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := importWarehouse(app, source); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := app.FindFirstRecordByFilter("warehouse_settings", "key = '__import_manifest'"); err != nil {
		t.Fatalf("completion marker: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "secret" {
		t.Fatalf("outside file changed: %q %v", data, err)
	}
}

func TestWarehouseImportRetriesAfterWriteFailure(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	source := t.TempDir()
	for name, content := range map[string]string{"a.txt": "first", "b.txt": "second"} {
		warehouseTestFile(t, source, "demo", "docs", name, []byte(content))
	}
	fail := true
	app.OnRecordCreate("warehouse_files").BindFunc(func(e *core.RecordEvent) error {
		if fail && e.Record.GetString("name") == "b.txt" {
			return errors.New("injected write failure")
		}
		return e.Next()
	})
	if err := importWarehouse(app, source); err == nil {
		t.Fatal("write failure ignored")
	}
	if _, err := app.FindFirstRecordByFilter("warehouse_settings", "key = '__import_manifest'"); err == nil {
		t.Fatal("failed import marked complete")
	}
	fail = false
	if err := importWarehouse(app, source); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := app.FindFirstRecordByFilter("warehouse_settings", "key = '__import_manifest'"); err != nil {
		t.Fatal(err)
	}
}
