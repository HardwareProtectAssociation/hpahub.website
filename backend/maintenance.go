package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
)

// maintenanceCommand runs before PocketBase initialization; these commands never start a server.
func maintenanceCommand(args []string) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "hpa-validate-restore":
		if len(args) != 3 {
			return true, errors.New("usage: hpa-validate-restore ZIP DEST")
		}
		if err := maintenanceValidateRestore(args[1], args[2]); err != nil {
			return true, fmt.Errorf("备份验证失败：%w", err)
		}
		return true, nil
	case "hpa-legacy-snapshot":
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
		gpu := flags.String("gpu-db", "", "legacy GPU SQLite database")
		warehouse := flags.String("warehouse-root", "", "legacy warehouse directory")
		auth := flags.String("auth-db", "", "legacy account SQLite database")
		output := flags.String("output", "", "new private snapshot directory (must not exist)")
		if err := flags.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return true, nil
			}
			return true, err
		}
		if flags.NArg() != 0 || *gpu == "" || *warehouse == "" || *auth == "" || *output == "" {
			return true, errors.New("hpa-legacy-snapshot requires --gpu-db, --warehouse-root, --auth-db and --output")
		}
		if err := maintenanceSnapshot(*gpu, *warehouse, *auth, *output); err != nil {
			return true, fmt.Errorf("snapshot failed: %w", err)
		}
		return true, nil
	default:
		return false, nil
	}
}

func maintenanceOpenDB(path string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Keep errors explicit instead of accepting unsafe platform-specific fallbacks.
var errMaintenanceUnsupported = fmt.Errorf("maintenance commands require Linux (run the application container)")
