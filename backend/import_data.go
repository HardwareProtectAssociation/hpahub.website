package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Data imports run offline against a separate PocketBase data directory.
func registerDataImportCommands(app core.App, root *cobra.Command) {
	for _, spec := range []struct {
		name       string
		importData func(core.App, string) error
	}{
		{"hpa-import-gpu", importGPU},
		{"hpa-import-warehouse", importWarehouse},
	} {
		var source string
		spec := spec
		cmd := &cobra.Command{
			Use:  spec.name,
			Args: cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				if err := app.RunAllMigrations(); err != nil {
					return err
				}
				return spec.importData(app, source)
			},
		}
		cmd.Flags().StringVar(&source, "source", "", "Path to the read-only legacy snapshot")
		_ = cmd.MarkFlagRequired("source")
		root.AddCommand(cmd)
	}
}

// Never serve an empty replacement while a legacy volume still needs importing.
func checkLegacyDataImported(app core.App, gpuPath, warehousePath string) error {
	for _, source := range []struct {
		path, label string
		ready       func() (bool, error)
	}{
		{gpuPath, "GPU", func() (bool, error) {
			var count int
			err := app.DB().NewQuery("SELECT count(*) FROM hpahub_gpu_import WHERE id = 1").Row(&count)
			return count == 1, err
		}},
		{warehousePath, "warehouse", func() (bool, error) {
			var count int
			err := app.DB().NewQuery("SELECT count(*) FROM warehouse_imports WHERE version = '1'").Row(&count)
			return count == 1, err
		}},
	} {
		entries, err := os.ReadDir(source.path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking legacy %s volume: %w", source.label, err)
		}
		if len(entries) == 0 {
			continue
		}
		ready, err := source.ready()
		if err != nil {
			return fmt.Errorf("checking legacy %s import: %w", source.label, err)
		}
		if !ready {
			return fmt.Errorf("legacy %s data has not been imported; stop services and follow the README migration procedure", source.label)
		}
	}
	return nil
}
