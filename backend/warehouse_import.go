package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func warehouseImportCheckContent(storage *filesystem.System, record *core.Record, path, digest string) error {
	name := record.GetString("content")
	if name == "" {
		return fmt.Errorf("warehouse file missing at %q", path)
	}
	reader, err := storage.GetReader(record.BaseFilesPath() + "/" + name)
	if err != nil {
		return fmt.Errorf("warehouse file %q: %w", path, err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return fmt.Errorf("warehouse content conflict at %q", path)
	}
	return nil
}

// importWarehouse reads the legacy volume without modifying it. Each completed
// record is independently retryable; mismatches are errors, never overwrites.
func importWarehouse(app core.App, source string) error {
	warehouseMutex.Lock()
	defer warehouseMutex.Unlock()

	root, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !root.IsDir() || root.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("warehouse source must be a real directory")
	}
	sourcePath, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return err
	}
	dataPath, err := filepath.Abs(app.DataDir())
	if err != nil {
		return err
	}
	dataPath, err = filepath.EvalSymlinks(dataPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.VolumeName(sourcePath), filepath.VolumeName(dataPath)) {
		relativeData, err := filepath.Rel(sourcePath, dataPath)
		if err != nil {
			return err
		}
		if relativeData == "." || (relativeData != ".." && !strings.HasPrefix(relativeData, ".."+string(filepath.Separator))) {
			return fmt.Errorf("warehouse source contains PocketBase data directory")
		}
	}
	collection, err := app.FindCollectionByNameOrId("warehouse_items")
	if err != nil {
		return err
	}
	storage, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer storage.Close()
	// Reject malformed input before creating any records.
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported warehouse entry %q", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(rel) > 4096 || !utf8.ValidString(rel) || strings.ContainsRune(rel, '\x00') {
			return fmt.Errorf("invalid warehouse path %q", rel)
		}
		if info.Mode().IsRegular() && (entry.Name() == ".meta.json" || rel == "storage_config.json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var object map[string]any
			if err := json.Unmarshal(data, &object); err != nil || object == nil {
				return fmt.Errorf("invalid warehouse JSON %q", rel)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	manifest := sha256.New()
	err = filepath.WalkDir(source, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported warehouse entry %q", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(rel) > 4096 || !utf8.ValidString(rel) || strings.ContainsRune(rel, '\x00') {
			return fmt.Errorf("invalid warehouse path %q", rel)
		}
		seen[rel] = true
		mode := strconv.FormatUint(uint64(info.Mode()), 10)
		modified := strconv.FormatInt(info.ModTime().UnixNano(), 10)
		created := ""
		if info.IsDir() {
			created = strconv.FormatInt(warehouseCreated(info).UnixNano(), 10)
		}
		digest := ""
		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			h := sha256.New()
			_, copyErr := io.Copy(h, f)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			digest = hex.EncodeToString(h.Sum(nil))
		}
		if err := json.NewEncoder(manifest).Encode([]any{rel, info.IsDir(), mode, modified, created, digest}); err != nil {
			return err
		}
		record, err := app.FindFirstRecordByFilter(collection, "path = {:path}", dbx.Params{"path": rel})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if record != nil {
			if record.GetBool("directory") != info.IsDir() || record.GetString("mode") != mode || record.GetString("modified") != modified || record.GetString("created") != created || record.GetString("digest") != digest {
				return fmt.Errorf("warehouse conflict at %q", rel)
			}
			if !info.IsDir() {
				if err := warehouseImportCheckContent(storage, record, rel, digest); err != nil {
					return err
				}
			} else if record.GetString("content") != "" {
				return fmt.Errorf("warehouse directory conflict at %q", rel)
			}
			return nil
		}
		record = core.NewRecord(collection)
		record.Set("path", rel)
		record.Set("directory", info.IsDir())
		record.Set("mode", mode)
		record.Set("modified", modified)
		record.Set("created", created)
		record.Set("digest", digest)
		if !info.IsDir() {
			file, err := filesystem.NewFileFromPath(path)
			if err != nil {
				return err
			}
			record.Set("content", file)
		}
		if err := app.Save(record); err != nil {
			return fmt.Errorf("import warehouse %q: %w", rel, err)
		}
		if !info.IsDir() {
			if err := warehouseImportCheckContent(storage, record, rel, digest); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	records, err := app.FindRecordsByFilter(collection, "", "", 0, 0)
	if err != nil {
		return err
	}
	for _, record := range records {
		if !seen[record.GetString("path")] {
			return fmt.Errorf("warehouse target contains extra path %q", record.GetString("path"))
		}
	}
	imports, err := app.FindCollectionByNameOrId("warehouse_imports")
	if err != nil {
		return err
	}
	marker, err := app.FindFirstRecordByFilter(imports, "version = '1'")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sum := hex.EncodeToString(manifest.Sum(nil))
	if marker != nil {
		if marker.GetString("manifest") != sum {
			return fmt.Errorf("warehouse import manifest conflict")
		}
		return nil
	}
	marker = core.NewRecord(imports)
	marker.Set("version", "1")
	marker.Set("manifest", sum)
	return app.Save(marker)
}
