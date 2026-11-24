package main

import (
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
