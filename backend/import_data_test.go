package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyVolumeRequiresImportBeforeServing(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	gpu, warehouse := filepath.Join(root, "gpu"), filepath.Join(root, "warehouse")
	for _, path := range []string{gpu, warehouse} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkLegacyDataImported(app, gpu, warehouse); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gpu, "gpu.db"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkLegacyDataImported(app, gpu, warehouse); err == nil || !strings.Contains(err.Error(), "GPU") {
		t.Fatalf("legacy GPU must block startup: %v", err)
	}
	if _, err := app.DB().NewQuery("INSERT INTO hpahub_gpu_import (id) VALUES (1)").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(warehouse, "storage_config.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkLegacyDataImported(app, gpu, warehouse); err == nil || !strings.Contains(err.Error(), "warehouse") {
		t.Fatalf("legacy warehouse must block startup: %v", err)
	}
	if _, err := app.DB().NewQuery("INSERT INTO warehouse_imports (id, version, manifest) VALUES ('testimport00001', '1', 'ok')").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := checkLegacyDataImported(app, gpu, warehouse); err != nil {
		t.Fatal(err)
	}
}
