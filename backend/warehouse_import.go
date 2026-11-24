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
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type warehouseImportFile struct {
	project, category, name string
	paths                   map[string]string
	digests                 map[string]string
	metadata                map[string]any
}

func warehouseImportDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func warehouseImportEqualJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var x, y any
	if json.Unmarshal(left, &x) != nil || json.Unmarshal(right, &y) != nil {
		return false
	}
	left, _ = json.Marshal(x)
	right, _ = json.Marshal(y)
	return string(left) == string(right)
}

func warehouseImportCheckContent(storage *filesystem.System, record *core.Record, field, path, digest string) error {
	name := record.GetString(field)
	if name == "" {
		if digest == "" {
			return nil
		}
		return fmt.Errorf("warehouse file missing at %q", path)
	}
	if digest == "" {
		return fmt.Errorf("warehouse unexpected %s at %q", field, path)
	}
	reader, err := storage.GetReader(record.BaseFilesPath() + "/" + name)
	if err != nil {
		return fmt.Errorf("warehouse file %q: %w", path, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("warehouse content conflict at %q", path)
	}
	return nil
}

// importWarehouse copies visible legacy data without modifying the source.
// Records already copied by a failed import are verified before reuse.
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
		relative, err := filepath.Rel(sourcePath, dataPath)
		if err != nil {
			return err
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return fmt.Errorf("warehouse source contains PocketBase data directory")
		}
	}

	// Validate the entire tree before writing anything, including ignored entries.
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if len(rel) > 4096 || !utf8.ValidString(rel) || strings.ContainsRune(rel, '\x00') {
			return fmt.Errorf("invalid warehouse path %q", rel)
		}
		return nil
	}); err != nil {
		return err
	}

	readObject := func(path string) (map[string]any, bool, error) {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil || value == nil {
			return nil, false, fmt.Errorf("invalid warehouse JSON %q", path)
		}
		return value, true, nil
	}
	config, hasConfig, err := readObject(filepath.Join(source, "storage_config.json"))
	if err != nil {
		return err
	}
	projects := map[string]map[string]any{}
	files := []warehouseImportFile{}
	dirs, err := os.ReadDir(filepath.Join(source, "projects"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, dir := range dirs {
		if !dir.IsDir() || strings.HasPrefix(dir.Name(), ".") {
			continue
		}
		name, err := warehouseName(dir.Name())
		if err != nil || name != dir.Name() {
			return fmt.Errorf("invalid warehouse project name %q", dir.Name())
		}
		base := filepath.Join(source, "projects", name)
		metadata, hasMetadata, err := readObject(filepath.Join(base, ".meta.json"))
		if err != nil {
			return err
		}
		start := len(files)
		for _, category := range warehouseCategories {
			entries, err := os.ReadDir(filepath.Join(base, category))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				filename, err := warehouseName(entry.Name())
				if err != nil || filename != entry.Name() {
					return fmt.Errorf("invalid warehouse filename %q", entry.Name())
				}
				path := filepath.Join(base, category, filename)
				info, err := entry.Info()
				if err != nil {
					return err
				}
				item := warehouseImportFile{project: name, category: category, name: filename, paths: map[string]string{"content": path}, digests: map[string]string{}, metadata: map[string]any{"modified": warehouseTime(info.ModTime())}}
				if category == "videos" {
					for field, sidecar := range map[string]string{"thumbnail": filepath.Join(".thumbs", filename+".jpg"), "web": filepath.Join(".web", filename), "preview": filepath.Join(".preview", filename)} {
						path := filepath.Join(base, category, sidecar)
						info, err := os.Stat(path)
						if errors.Is(err, os.ErrNotExist) {
							continue
						}
						if err != nil {
							return err
						}
						if !info.Mode().IsRegular() {
							return fmt.Errorf("invalid warehouse sidecar %q", path)
						}
						item.paths[field] = path
					}
				}
				for field, path := range item.paths {
					digest, err := warehouseImportDigest(path)
					if err != nil {
						return err
					}
					item.digests[field] = digest
				}
				files = append(files, item)
			}
		}
		if hasMetadata || len(files) != start {
			if metadata["created"] == nil || metadata["created"] == "" {
				info, err := dir.Info()
				if err != nil {
					return err
				}
				metadata["created"] = warehouseTime(warehouseCreated(info))
			}
			projects[name] = metadata
		}
	}

	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	encoder := json.NewEncoder(hash)
	if err := encoder.Encode([]any{projects, hasConfig, config}); err != nil {
		return err
	}
	for _, item := range files {
		if err := encoder.Encode([]any{item.project, item.category, item.name, item.digests, item.metadata}); err != nil {
			return err
		}
	}
	manifest := hex.EncodeToString(hash.Sum(nil))
	projectCollection, err := app.FindCollectionByNameOrId("warehouse_projects")
	if err != nil {
		return err
	}
	fileCollection, err := app.FindCollectionByNameOrId("warehouse_files")
	if err != nil {
		return err
	}
	settingsCollection, err := app.FindCollectionByNameOrId("warehouse_settings")
	if err != nil {
		return err
	}
	find := func(collection *core.Collection, filter string, params dbx.Params) (*core.Record, error) {
		record, err := app.FindFirstRecordByFilter(collection, filter, params)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return record, err
	}
	marker, err := find(settingsCollection, "key = {:key}", dbx.Params{"key": "__import_manifest"})
	if err != nil {
		return err
	}
	if marker != nil && !warehouseImportEqualJSON(marker.Get("value"), manifest) {
		return fmt.Errorf("warehouse import manifest conflict")
	}
	storage, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer storage.Close()
	projectRecords := map[string]*core.Record{}
	for _, name := range names {
		record, err := find(projectCollection, "name = {:name}", dbx.Params{"name": name})
		if err != nil {
			return err
		}
		if record != nil {
			if !warehouseImportEqualJSON(record.Get("metadata"), projects[name]) {
				return fmt.Errorf("warehouse project conflict %q", name)
			}
		} else {
			record = core.NewRecord(projectCollection)
			record.Set("name", name)
			record.Set("metadata", projects[name])
			if err := app.Save(record); err != nil {
				return fmt.Errorf("import warehouse project %q: %w", name, err)
			}
		}
		projectRecords[name] = record
	}
	for _, item := range files {
		project := projectRecords[item.project]
		record, err := find(fileCollection, "project = {:project} && category = {:category} && name = {:name}", dbx.Params{"project": project.Id, "category": item.category, "name": item.name})
		if err != nil {
			return err
		}
		if record != nil {
			if !warehouseImportEqualJSON(record.Get("metadata"), item.metadata) {
				return fmt.Errorf("warehouse file metadata conflict %q", item.paths["content"])
			}
		} else {
			record = core.NewRecord(fileCollection)
			record.Set("project", project.Id)
			record.Set("category", item.category)
			record.Set("name", item.name)
			record.Set("metadata", item.metadata)
			for field, path := range item.paths {
				file, err := filesystem.NewFileFromPath(path)
				if err != nil {
					return err
				}
				record.Set(field, file)
			}
			if err := app.Save(record); err != nil {
				return fmt.Errorf("import warehouse file %q: %w", item.paths["content"], err)
			}
		}
		for _, field := range []string{"content", "thumbnail", "web", "preview"} {
			if err := warehouseImportCheckContent(storage, record, field, item.paths["content"], item.digests[field]); err != nil {
				return err
			}
		}
	}
	if hasConfig {
		record, err := find(settingsCollection, "key = {:key}", dbx.Params{"key": "storage_config"})
		if err != nil {
			return err
		}
		if record != nil {
			if !warehouseImportEqualJSON(record.Get("value"), config) {
				return fmt.Errorf("warehouse storage configuration conflict")
			}
		} else {
			record = core.NewRecord(settingsCollection)
			record.Set("key", "storage_config")
			record.Set("value", config)
			if err := app.Save(record); err != nil {
				return err
			}
		}
	}
	expectedSettings := 0
	if hasConfig {
		expectedSettings++
	}
	if marker != nil {
		expectedSettings++
	}
	for _, check := range []struct {
		collection *core.Collection
		expected   int
	}{
		{projectCollection, len(projects)},
		{fileCollection, len(files)},
		{settingsCollection, expectedSettings},
	} {
		records, err := app.FindRecordsByFilter(check.collection, "", "", 0, 0)
		if err != nil {
			return err
		}
		if len(records) != check.expected {
			return fmt.Errorf("warehouse target contains extra records in %s", check.collection.Name)
		}
	}
	if marker == nil {
		marker = core.NewRecord(settingsCollection)
		marker.Set("key", "__import_manifest")
		encoded, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		marker.Set("value", string(encoded))
		return app.Save(marker)
	}
	return nil
}
