package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const warehouseBodyLimit int64 = 500 << 20

var warehouseCategories = []string{"images", "videos", "docs", "others"}

// ponytail: serialize filesystem API operations; use per-project locks if contention matters.
var warehouseMutex sync.Mutex

type warehouse struct {
	app core.App
}

type warehouseError struct {
	status  int
	message string
}

func (e *warehouseError) Error() string { return e.message }

func warehouseFailure(status int, message string) error {
	return &warehouseError{status: status, message: message}
}

func warehouseJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func warehouseWriteError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "服务器文件操作失败"
	var failure *warehouseError
	var tooLarge *http.MaxBytesError
	if errors.As(err, &failure) {
		status, message = failure.status, failure.message
	} else if errors.As(err, &tooLarge) || errors.Is(err, apis.ErrRequestEntityTooLarge) {
		status, message = http.StatusRequestEntityTooLarge, "上传内容超过 500 MiB 限制"
	}
	warehouseJSON(w, status, map[string]any{"ok": false, "error": message})
}

func warehouseName(value any) (string, error) {
	name, ok := value.(string)
	if !ok {
		return "", warehouseFailure(400, "名称必须是字符串")
	}
	name = strings.TrimSpace(name)
	message := ""
	switch {
	case name == "":
		message = "名称不能为空"
	case utf8.RuneCountInString(name) > 255:
		message = "名称过长（最多 255 个字符）"
	case strings.ContainsAny(name, "/\\\x00"):
		message = "名称不能包含路径分隔符"
	case name == "." || name == "..":
		message = "名称非法"
	case strings.ContainsAny(name, "<>:\"|?*"):
		message = "名称不能包含 < > : \" | ? * 等非法字符"
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		message = "名称不能以点或空格结尾"
	}
	for _, char := range name {
		if char < 32 {
			message = "名称包含非法控制字符"
			break
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
		(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
		message = "名称为系统保留名，请更换"
	}
	if message != "" {
		return "", warehouseFailure(400, message)
	}
	if strings.HasPrefix(name, ".") {
		return "", warehouseFailure(400, "名称非法")
	}
	return name, nil
}

// Paths are collection keys, never host filesystem paths.
func (s warehouse) path(parts ...string) (string, error) {
	key := "projects"
	for _, part := range parts {
		record, err := s.record(key)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if record != nil && !record.GetBool("directory") {
			return "", warehouseFailure(400, "非法路径")
		}
		key = path.Join(key, part)
	}
	return key, nil
}

func (s warehouse) record(key string) (*core.Record, error) {
	record, err := s.app.FindFirstRecordByFilter("warehouse_items", "path = {:path}", dbx.Params{"path": key})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fs.ErrNotExist
	}
	return record, err
}

func (s warehouse) children(key string) ([]*core.Record, error) {
	records, err := s.app.FindRecordsByFilter("warehouse_items", "path ~ {:prefix}", "path", 0, 0, dbx.Params{"prefix": key + "/"})
	if err != nil {
		return nil, err
	}
	result := make([]*core.Record, 0, len(records))
	for _, record := range records {
		rel := strings.TrimPrefix(record.GetString("path"), key+"/")
		if rel != record.GetString("path") && !strings.Contains(rel, "/") {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s warehouse) mkdir(key string) error {
	if _, err := s.record(key); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	collection, err := s.app.FindCollectionByNameOrId("warehouse_items")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("path", key)
	record.Set("directory", true)
	record.Set("mode", strconv.FormatUint(uint64(os.ModeDir|0755), 10))
	now := time.Now()
	record.Set("modified", strconv.FormatInt(now.UnixNano(), 10))
	record.Set("created", strconv.FormatInt(now.UnixNano(), 10))
	return s.app.Save(record)
}

func (s warehouse) ensureDir(key string) error {
	record, err := s.record(key)
	if errors.Is(err, fs.ErrNotExist) {
		return s.mkdir(key)
	}
	if err != nil {
		return err
	}
	if !record.GetBool("directory") {
		return warehouseFailure(400, "非法路径")
	}
	return nil
}

func (s warehouse) saveFile(key, source string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	record, err := s.record(key)
	if errors.Is(err, fs.ErrNotExist) {
		collection, e := s.app.FindCollectionByNameOrId("warehouse_items")
		if e != nil {
			return e
		}
		record = core.NewRecord(collection)
		record.Set("path", key)
	} else if err != nil {
		return err
	} else if record.GetBool("directory") {
		return warehouseFailure(400, "非法路径")
	}
	content, err := filesystem.NewFileFromPath(source)
	if err != nil {
		return err
	}
	record.Set("directory", false)
	record.Set("mode", strconv.FormatUint(uint64(0644), 10))
	record.Set("modified", strconv.FormatInt(time.Now().UnixNano(), 10))
	record.Set("digest", hex.EncodeToString(hash.Sum(nil)))
	record.Set("content", content)
	return s.app.Save(record)
}

func (s warehouse) temporary(key string) (string, error) {
	record, err := s.record(key)
	if err != nil {
		return "", err
	}
	if record.GetBool("directory") {
		return "", fs.ErrNotExist
	}
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return "", err
	}
	defer storage.Close()
	reader, err := storage.GetReader(record.BaseFilesPath() + "/" + record.GetString("content"))
	if err != nil {
		return "", err
	}
	defer reader.Close()
	file, err := os.CreateTemp("", "warehouse-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	_, err = io.Copy(file, reader)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func warehouseModified(record *core.Record) time.Time {
	nanos, _ := strconv.ParseInt(record.GetString("modified"), 10, 64)
	return time.Unix(0, nanos)
}

func (s warehouse) size(record *core.Record) (int64, error) {
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return 0, err
	}
	defer storage.Close()
	attributes, err := storage.Attributes(record.BaseFilesPath() + "/" + record.GetString("content"))
	if err != nil {
		return 0, err
	}
	return attributes.Size, nil
}

func (s warehouse) tree(key string) ([]*core.Record, error) {
	record, err := s.record(key)
	if err != nil {
		return nil, err
	}
	records, err := s.app.FindRecordsByFilter("warehouse_items", "path ~ {:prefix}", "path", 0, 0, dbx.Params{"prefix": key + "/"})
	if err != nil {
		return nil, err
	}
	result := []*core.Record{record}
	for _, child := range records {
		if strings.HasPrefix(child.GetString("path"), key+"/") {
			result = append(result, child)
		}
	}
	return result, nil
}

func (s warehouse) moveTree(source, destination string) error {
	return s.app.RunInTransaction(func(app core.App) error {
		tx := warehouse{app: app}
		records, err := tx.tree(source)
		if err != nil {
			return err
		}
		for _, record := range records {
			record.Set("path", destination+strings.TrimPrefix(record.GetString("path"), source))
			if err := app.Save(record); err != nil {
				return err
			}
		}
		return nil
	})
}

const warehouseDeletedPrefix = "/.warehouse-deleted/"

func (s warehouse) cleanupDeleted() error {
	records, err := s.app.FindRecordsByFilter("warehouse_items", "path ~ {:prefix}", "", 0, 0, dbx.Params{"prefix": warehouseDeletedPrefix})
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return err
	}
	defer storage.Close()
	for _, record := range records {
		if !strings.HasPrefix(record.GetString("path"), warehouseDeletedPrefix) {
			continue
		}
		if failures := storage.DeletePrefix(record.BaseFilesPath() + "/"); len(failures) != 0 {
			return errors.Join(failures...)
		}
		if _, err := s.app.DB().Delete("warehouse_items", dbx.HashExp{"id": record.Id}).Execute(); err != nil {
			return err
		}
	}
	return nil
}

func (s warehouse) deleteTree(keys ...string) error {
	var records []*core.Record
	for _, key := range keys {
		items, err := s.tree(key)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		records = append(records, items...)
	}
	// PocketBase deletes files asynchronously after record deletion. Mark files
	// in the database first, then clean synchronously under warehouseMutex.
	err := s.app.RunInTransaction(func(app core.App) error {
		for _, record := range records {
			var err error
			if record.GetString("content") == "" {
				_, err = app.DB().Delete("warehouse_items", dbx.HashExp{"id": record.Id}).Execute()
			} else {
				_, err = app.DB().Update("warehouse_items", dbx.Params{"path": warehouseDeletedPrefix + record.Id}, dbx.HashExp{"id": record.Id}).Execute()
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.cleanupDeleted()
}

func (s warehouse) project(name string) (string, error) {
	_, err := warehouseName(name)
	if err != nil {
		return "", err
	}
	return s.path(name)
}

func (s warehouse) checkedFile(project, category, filename, kind string) (string, error) {
	if _, err := s.project(project); err != nil {
		return "", err
	}
	_, err := warehouseName(filename)
	if err != nil {
		return "", err
	}
	if !warehouseCategory(category) {
		return "", warehouseFailure(400, "非法文件分类")
	}
	parts := []string{project, category}
	switch kind {
	case "file":
	case "thumb", "web", "preview":
		if category != "videos" {
			return "", warehouseFailure(400, "非法媒体类型")
		}
		parts = append(parts, map[string]string{"thumb": ".thumbs", "web": ".web", "preview": ".preview"}[kind])
		if kind == "thumb" {
			filename += ".jpg"
		}
	default:
		return "", warehouseFailure(400, "非法媒体类型")
	}
	return s.path(append(parts, filename)...)
}

func warehouseCategory(category string) bool {
	for _, candidate := range warehouseCategories {
		if category == candidate {
			return true
		}
	}
	return false
}

func warehouseClassify(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg":
		return "images"
	case "mp4", "avi", "mkv", "mov", "wmv", "flv", "webm":
		return "videos"
	case "txt", "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv":
		return "docs"
	default:
		return "others"
	}
}

func warehouseMediaURL(kind, project, category, filename string) string {
	parts := []string{kind, project, category, filename}
	for i := range parts {
		parts[i] = strings.ReplaceAll(url.QueryEscape(parts[i]), "+", "%20")
	}
	return "/api/hpahub/warehouse/media/" + strings.Join(parts, "/")
}

func warehouseIdentity(r *http.Request) map[string]string {
	id := strings.TrimSpace(r.Header.Get("X-User-Id"))
	name := strings.TrimSpace(r.Header.Get("X-User-Name"))
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	if id == "" && name == "" {
		return nil
	}
	if id == "" {
		id = name
	}
	if name == "" {
		name = id
	}
	return map[string]string{"id": id, "name": name}
}

func (s warehouse) readJSON(key string) (map[string]any, error) {
	result := map[string]any{}
	record, err := s.record(key)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if record.GetBool("directory") {
		return nil, warehouseFailure(400, "非法路径")
	}
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return nil, err
	}
	defer storage.Close()
	reader, err := storage.GetReader(record.BaseFilesPath() + "/" + record.GetString("content"))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func (s warehouse) saveJSON(key string, data map[string]any) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("", "warehouse-json-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(encoded)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return s.saveFile(key, file.Name())
}

func (s warehouse) meta(project string) (map[string]any, error) {
	key, err := s.path(project, ".meta.json")
	if err != nil {
		return nil, err
	}
	return s.readJSON(key)
}

func (s warehouse) saveMeta(project string, data map[string]any) error {
	key, err := s.path(project, ".meta.json")
	if err != nil {
		return err
	}
	return s.saveJSON(key, data)
}

func warehouseTime(t time.Time) string { return t.Local().Format("2006-01-02T15:04:05") }

func (s warehouse) regular(key string) bool {
	record, err := s.record(key)
	return err == nil && !record.GetBool("directory")
}

func (s warehouse) fileEntry(ctx context.Context, project, category, name, key string) (map[string]any, error) {
	record, err := s.record(key)
	if err != nil {
		return nil, err
	}
	size, err := s.size(record)
	if err != nil {
		return nil, err
	}
	entry := map[string]any{"name": name, "category": category, "size": size, "modified": warehouseTime(warehouseModified(record)), "url": warehouseMediaURL("file", project, category, name)}
	if category == "videos" {
		for kind, field := range map[string]string{"thumb": "thumb", "web": "web_url", "preview": "preview_url"} {
			sidecar, err := s.checkedFile(project, category, name, kind)
			if err == nil && s.regular(sidecar) {
				entry[field] = warehouseMediaURL(kind, project, category, name)
			}
		}
		temporary, err := s.temporary(key)
		if err != nil {
			return nil, err
		}
		entry["codec"] = warehouseCodec(ctx, temporary)
		os.Remove(temporary)
	}
	return entry, nil
}

func (s warehouse) files(ctx context.Context, project string) (map[string][]map[string]any, error) {
	key, err := s.project(project)
	if err != nil {
		return nil, err
	}
	record, err := s.record(key)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !record.GetBool("directory")) {
		return nil, warehouseFailure(404, "项目不存在")
	}
	if err != nil {
		return nil, err
	}
	result := map[string][]map[string]any{}
	for _, category := range warehouseCategories {
		result[category] = []map[string]any{}
		dir, err := s.path(project, category)
		if err != nil {
			return nil, err
		}
		entries, err := s.children(dir)
		if err != nil {
			return nil, err
		}
		for _, item := range entries {
			name := path.Base(item.GetString("path"))
			if strings.HasPrefix(name, ".") || item.GetBool("directory") {
				continue
			}
			key, err := s.checkedFile(project, category, name, "file")
			if err != nil {
				continue
			}
			entry, err := s.fileEntry(ctx, project, category, name, key)
			if err != nil {
				return nil, err
			}
			result[category] = append(result[category], entry)
		}
	}
	return result, nil
}

func (s warehouse) summary(project string) (map[string]any, error) {
	key, err := s.project(project)
	if err != nil {
		return nil, err
	}
	meta, err := s.meta(project)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	var size int64
	total := 0
	for _, category := range warehouseCategories {
		counts[category] = 0
		dir, err := s.path(project, category)
		if err != nil {
			return nil, err
		}
		entries, err := s.children(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if strings.HasPrefix(path.Base(entry.GetString("path")), ".") || entry.GetBool("directory") {
				continue
			}
			fileSize, err := s.size(entry)
			if err != nil {
				return nil, err
			}
			counts[category]++
			total++
			size += fileSize
		}
	}
	created := meta["created"]
	if created == nil || created == "" {
		if record, err := s.record(key); err == nil {
			nanos, err := strconv.ParseInt(record.GetString("created"), 10, 64)
			if err == nil {
				created = warehouseTime(time.Unix(0, nanos))
			} else {
				created = warehouseTime(warehouseModified(record))
			}
		}
	}
	display := meta["display_name"]
	if display == nil {
		display = project
	}
	return map[string]any{"name": project, "display_name": display, "created": created, "counts": counts, "total_files": total, "size_bytes": size, "creator_id": meta["creator_id"], "creator_name": meta["creator_name"]}, nil
}

func warehouseRequestJSON(r *http.Request) (map[string]any, error) {
	data := map[string]any{}
	err := json.NewDecoder(r.Body).Decode(&data)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) || errors.Is(err, apis.ErrRequestEntityTooLarge) {
		return nil, err
	}
	if data == nil || err != nil {
		data = map[string]any{}
	}
	return data, nil
}

func serveWarehouse(app core.App, w http.ResponseWriter, r *http.Request) {
	s := warehouse{app: app}
	if r.ContentLength > warehouseBodyLimit {
		warehouseWriteError(w, &http.MaxBytesError{Limit: warehouseBodyLimit})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, warehouseBodyLimit)
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	warehouseMutex.Lock()
	defer warehouseMutex.Unlock()
	if err := s.cleanupDeleted(); err != nil {
		warehouseWriteError(w, err)
		return
	}
	if len(parts) == 7 && parts[0] == "api" && parts[1] == "hpahub" && parts[2] == "media" && (r.Method == "GET" || r.Method == "HEAD") {
		s.serveMedia(w, r, parts[3], parts[4], parts[5], parts[6])
		return
	}
	for _, key := range []string{".", "projects"} {
		if err := s.ensureDir(key); err != nil {
			warehouseWriteError(w, err)
			return
		}
	}
	data, status, err := s.api(r, parts)
	if err != nil {
		warehouseWriteError(w, err)
		return
	}
	warehouseJSON(w, status, data)
}

func (s warehouse) api(r *http.Request, parts []string) (any, int, error) {
	ok := map[string]any{"ok": true}
	if len(parts) < 2 || parts[0] != "api" {
		return nil, 0, warehouseFailure(404, "Not found.")
	}
	if len(parts) == 2 && parts[1] == "storage" {
		return s.storage(r)
	}
	if parts[1] != "projects" {
		return nil, 0, warehouseFailure(404, "Not found.")
	}
	if len(parts) == 2 {
		switch r.Method {
		case "GET":
			entries, err := s.children("projects")
			if err != nil {
				return nil, 0, err
			}
			projects := []map[string]any{}
			for _, entry := range entries {
				name := path.Base(entry.GetString("path"))
				if !entry.GetBool("directory") || strings.HasPrefix(name, ".") {
					continue
				}
				project, err := s.summary(name)
				if err != nil {
					return nil, 0, err
				}
				projects = append(projects, project)
			}
			return map[string]any{"ok": true, "projects": projects, "disk": warehouseDiskUsage(s.app.DataDir()), "me": warehouseIdentity(r)}, 200, nil
		case "POST":
			data, err := warehouseRequestJSON(r)
			if err != nil {
				return nil, 0, err
			}
			name, err := warehouseName(warehouseNameField(data))
			if err != nil {
				return nil, 0, err
			}
			path, err := s.project(name)
			if err != nil {
				return nil, 0, err
			}
			if err = s.mkdir(path); errors.Is(err, fs.ErrExist) {
				return nil, 0, warehouseFailure(409, "同名项目已存在")
			}
			if err != nil {
				return nil, 0, err
			}
			for _, category := range warehouseCategories {
				if err := s.mkdir(path + "/" + category); err != nil {
					return nil, 0, err
				}
			}
			meta := map[string]any{"display_name": name, "created": warehouseTime(time.Now())}
			if identity := warehouseIdentity(r); identity != nil {
				meta["creator_id"], meta["creator_name"] = identity["id"], identity["name"]
			}
			if err := s.saveMeta(name, meta); err != nil {
				return nil, 0, err
			}
			project, err := s.summary(name)
			return map[string]any{"ok": true, "project": project}, 201, err
		}
		return nil, 0, warehouseFailure(405, "Method not allowed.")
	}
	project := parts[2]
	path, err := s.project(project)
	if err != nil {
		return nil, 0, err
	}
	info, err := s.record(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.GetBool("directory")) {
		return nil, 0, warehouseFailure(404, "项目不存在")
	}
	if err != nil {
		return nil, 0, err
	}
	if len(parts) == 3 {
		switch r.Method {
		case "GET":
			files, err := s.files(r.Context(), project)
			if err != nil {
				return nil, 0, err
			}
			summary, err := s.summary(project)
			return map[string]any{"ok": true, "project": summary, "files": files}, 200, err
		case "PUT", "PATCH":
			data, err := warehouseRequestJSON(r)
			if err != nil {
				return nil, 0, err
			}
			name, err := warehouseName(warehouseNameField(data))
			if err != nil {
				return nil, 0, err
			}
			if name != project {
				destination, err := s.project(name)
				if err != nil {
					return nil, 0, err
				}
				if _, err := s.record(destination); err == nil {
					return nil, 0, warehouseFailure(409, "同名项目已存在")
				} else if !errors.Is(err, fs.ErrNotExist) {
					return nil, 0, err
				}
				if err := s.moveTree(path, destination); err != nil {
					return nil, 0, err
				}
				meta, err := s.meta(name)
				if err != nil {
					return nil, 0, err
				}
				meta["display_name"], meta["renamed"] = name, warehouseTime(time.Now())
				if err := s.saveMeta(name, meta); err != nil {
					return nil, 0, err
				}
			}
			summary, err := s.summary(name)
			return map[string]any{"ok": true, "project": summary}, 200, err
		case "DELETE":
			return ok, 200, s.deleteTree(path)
		}
	}
	if len(parts) == 4 && parts[3] == "files" && r.Method == "POST" {
		files, err := s.upload(r, project)
		return map[string]any{"ok": true, "files": files}, 201, err
	}
	if (len(parts) == 6 || len(parts) == 7) && parts[3] == "files" {
		return s.fileAPI(r, project, parts[4], parts[5], parts[6:])
	}
	return nil, 0, warehouseFailure(404, "Not found.")
}

func warehouseNameField(data map[string]any) any {
	if value, exists := data["name"]; exists {
		return value
	}
	return ""
}

func (s warehouse) upload(r *http.Request, project string) ([]map[string]any, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || errors.Is(err, apis.ErrRequestEntityTooLarge) {
			return nil, err
		}
		return nil, warehouseFailure(400, "没有收到文件")
	}
	defer r.MultipartForm.RemoveAll()
	uploads := r.MultipartForm.File["files"]
	names := r.MultipartForm.Value["names"]
	if len(uploads) == 0 {
		return nil, warehouseFailure(400, "没有收到文件")
	}
	saved := []map[string]any{}
	for index, upload := range uploads {
		original := strings.ReplaceAll(upload.Filename, "\\", "/")
		original = original[strings.LastIndex(original, "/")+1:]
		if original == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(original), ".") {
			return nil, warehouseFailure(400, "名称非法")
		}
		name, err := warehouseName(original)
		if err != nil {
			ext := ""
			if dot := strings.LastIndex(original, "."); dot >= 0 {
				runes := []rune(original[dot+1:])
				if len(runes) > 16 {
					runes = runes[:16]
				}
				ext = "." + string(runes)
			}
			name, err = warehouseName(fmt.Sprintf("file_%d%s", index, ext))
			if err != nil {
				return nil, err
			}
		}
		if index < len(names) && strings.TrimSpace(names[index]) != "" {
			name, err = warehouseName(names[index])
			if err != nil {
				return nil, warehouseFailure(400, fmt.Sprintf("文件名「%s」无效: %s", names[index], err))
			}
		}
		category := warehouseClassify(name)
		directory, err := s.path(project, category)
		if err != nil {
			return nil, err
		}
		if err := s.ensureDir(directory); err != nil {
			return nil, err
		}
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		var path string
		for counter := 0; ; counter++ {
			candidate := name
			if counter > 0 {
				candidate = stem + "_" + strconv.Itoa(counter) + ext
			}
			path = directory + "/" + candidate
			_, err = s.record(path)
			if err == nil {
				continue
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			name = candidate
			break
		}
		source, err := upload.Open()
		if err != nil {
			return nil, err
		}
		target, err := os.CreateTemp("", "warehouse-upload-*")
		if err != nil {
			source.Close()
			return nil, err
		}
		temporary := target.Name()
		_, err = io.Copy(target, source)
		sourceErr := source.Close()
		closeErr := target.Close()
		if err == nil {
			err = sourceErr
		}
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = s.saveFile(path, temporary)
		}
		os.Remove(temporary)
		if err != nil {
			return nil, err
		}
		entry, err := s.fileEntry(r.Context(), project, category, name, path)
		if err != nil {
			return nil, err
		}
		saved = append(saved, entry)
		if category == "videos" {
			_, _ = s.generateMedia(r.Context(), project, category, name, "thumb")
			_, _ = s.generateMedia(r.Context(), project, category, name, "preview")
		}
	}
	if len(saved) == 0 {
		return nil, warehouseFailure(400, "没有收到有效文件")
	}
	return saved, nil
}

func (s warehouse) fileAPI(r *http.Request, project, category, name string, suffix []string) (any, int, error) {
	path, err := s.checkedFile(project, category, name, "file")
	if err != nil {
		return nil, 0, err
	}
	if !s.regular(path) {
		status := 400
		if r.Method == "DELETE" {
			status = 404
		}
		return nil, 0, warehouseFailure(status, "文件不存在")
	}
	if len(suffix) == 1 && suffix[0] == "transcode" && r.Method == "POST" {
		if _, err := s.generateMedia(r.Context(), project, category, name, "web"); err != nil {
			return nil, 0, err
		}
		return map[string]any{"ok": true, "web_url": warehouseMediaURL("web", project, category, name)}, 200, nil
	}
	if len(suffix) != 0 {
		return nil, 0, warehouseFailure(404, "Not found.")
	}
	switch r.Method {
	case "DELETE":
		keys := []string{path}
		if category == "videos" {
			for _, kind := range []string{"thumb", "web", "preview"} {
				sidecar, err := s.checkedFile(project, category, name, kind)
				if err != nil {
					return nil, 0, err
				}
				record, err := s.record(sidecar)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					return nil, 0, err
				}
				if record.GetBool("directory") {
					return nil, 0, warehouseFailure(400, "非法路径")
				}
				keys = append(keys, sidecar)
			}
		}
		if err := s.deleteTree(keys...); err != nil {
			return nil, 0, err
		}
		return map[string]any{"ok": true}, 200, nil
	case "PUT", "PATCH":
		data, err := warehouseRequestJSON(r)
		if err != nil {
			return nil, 0, err
		}
		newName, err := warehouseName(warehouseNameField(data))
		if err != nil {
			return nil, 0, err
		}
		destination, err := s.checkedFile(project, category, newName, "file")
		if err != nil {
			return nil, 0, err
		}
		if newName != name {
			if _, err := s.record(destination); err == nil {
				return nil, 0, warehouseFailure(400, "同名文件已存在")
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, 0, err
			}
			moves := [][2]string{{path, destination}}
			if category == "videos" {
				for _, kind := range []string{"thumb", "web", "preview"} {
					oldSidecar, err := s.checkedFile(project, category, name, kind)
					if err != nil {
						return nil, 0, err
					}
					newSidecar, err := s.checkedFile(project, category, newName, kind)
					if err != nil {
						return nil, 0, err
					}
					record, err := s.record(oldSidecar)
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					if err != nil {
						return nil, 0, err
					}
					if record.GetBool("directory") {
						continue
					}
					if _, err := s.record(newSidecar); err == nil {
						return nil, 0, warehouseFailure(400, "同名文件已存在")
					} else if !errors.Is(err, fs.ErrNotExist) {
						return nil, 0, err
					}
					moves = append(moves, [2]string{oldSidecar, newSidecar})
				}
			}
			err = s.app.RunInTransaction(func(app core.App) error {
				tx := warehouse{app: app}
				for _, move := range moves {
					record, err := tx.record(move[0])
					if err != nil {
						return err
					}
					record.Set("path", move[1])
					if err := app.Save(record); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return nil, 0, err
			}
		}
		entry, err := s.fileEntry(r.Context(), project, category, newName, destination)
		return map[string]any{"ok": true, "file": entry}, 200, err
	}
	return nil, 0, warehouseFailure(405, "Method not allowed.")
}

func warehouseCodec(parent context.Context, path string) string {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "csv=p=0", path).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (s warehouse) generateMedia(parent context.Context, project, category, name, kind string) (string, error) {
	source, err := s.checkedFile(project, category, name, "file")
	if err != nil {
		return "", err
	}
	destination, err := s.checkedFile(project, category, name, kind)
	if err != nil {
		return "", err
	}
	if s.regular(destination) {
		return destination, nil
	}
	if err := s.ensureDir(path.Dir(destination)); err != nil {
		return "", err
	}
	source, err = s.temporary(source)
	if err != nil {
		return "", err
	}
	defer os.Remove(source)
	// Generate into a temporary file: failed transcodes never become cached previews.
	file, err := os.CreateTemp("", "warehouse-transcode-*")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	defer os.Remove(temporary)
	args := []string{"-y"}
	duration := 600 * time.Second
	switch kind {
	case "thumb":
		duration = 30 * time.Second
		args = append(args, "-ss", "1", "-i", source, "-vframes", "1", "-vf", "scale=480:-2", "-q:v", "4", "-f", "image2", temporary)
	case "web":
		args = append(args, "-i", source, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-vf", "scale='min(1280,iw)':-2", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-f", "mp4", temporary)
	case "preview":
		duration = 300 * time.Second
		args = append(args, "-i", source, "-vf", "scale=240:-2", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "36", "-an", "-movflags", "+faststart", "-f", "mp4", temporary)
	default:
		return "", warehouseFailure(400, "非法媒体类型")
	}
	ctx, cancel := context.WithTimeout(parent, duration)
	defer cancel()
	if err := exec.CommandContext(ctx, "ffmpeg", args...).Run(); err != nil {
		return "", warehouseFailure(400, "转码失败")
	}
	info, err := os.Stat(temporary)
	if err != nil || info.Size() == 0 {
		return "", warehouseFailure(400, "转码失败")
	}
	if err := s.saveFile(destination, temporary); err != nil {
		return "", err
	}
	return destination, nil
}

func (s warehouse) serveMedia(w http.ResponseWriter, r *http.Request, kind, project, category, name string) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	key, err := s.checkedFile(project, category, name, kind)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for parent := path.Dir(key); parent != "."; parent = path.Dir(parent) {
		directory, err := s.record(parent)
		if err != nil || !directory.GetBool("directory") {
			http.NotFound(w, r)
			return
		}
	}
	record, err := s.record(key)
	if err != nil || record.GetBool("directory") {
		http.NotFound(w, r)
		return
	}
	storage, err := s.app.NewFilesystem()
	if err != nil {
		http.Error(w, "服务器文件操作失败", 500)
		return
	}
	defer storage.Close()
	size, err := s.size(record)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(key))
	if kind == "thumb" {
		contentType = "image/jpeg"
	}
	if kind == "web" || kind == "preview" {
		contentType = "video/mp4"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	baseType, _, _ := mime.ParseMediaType(contentType)
	switch baseType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/webm", "video/ogg":
		w.Header().Set("Content-Disposition", "inline")
	default:
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%x-%x\"", warehouseModified(record).UnixNano(), size))
	if err := storage.Serve(w, r, record.BaseFilesPath()+"/"+record.GetString("content"), name); err != nil {
		http.NotFound(w, r)
	}
}

func (s warehouse) storage(r *http.Request) (any, int, error) {
	key := "storage_config.json"
	config, err := s.readJSON(key)
	if err != nil {
		return nil, 0, err
	}
	allocations, exists := config["allocations"]
	if !exists {
		allocations = map[string]any{}
	}
	switch r.Method {
	case "GET":
		usage := map[string]int64{}
		for _, category := range warehouseCategories {
			usage[category] = 0
		}
		entries, err := s.children("projects")
		if err != nil {
			return nil, 0, err
		}
		for _, entry := range entries {
			project := path.Base(entry.GetString("path"))
			if !entry.GetBool("directory") || strings.HasPrefix(project, ".") {
				continue
			}
			for _, category := range warehouseCategories {
				dir, err := s.path(project, category)
				if err != nil {
					return nil, 0, err
				}
				files, err := s.children(dir)
				if err != nil {
					return nil, 0, err
				}
				for _, file := range files {
					if file.GetBool("directory") {
						continue
					}
					size, err := s.size(file)
					if err != nil {
						return nil, 0, err
					}
					usage[category] += size
				}
			}
		}
		return map[string]any{"ok": true, "disks": warehouseDisks(), "categories": usage, "allocations": allocations}, 200, nil
	case "PUT", "PATCH":
		data, err := warehouseRequestJSON(r)
		if err != nil {
			return nil, 0, err
		}
		if value, exists := data["allocations"]; exists {
			allocations = value
		}
		config["allocations"] = allocations
		if err := s.saveJSON(key, config); err != nil {
			return nil, 0, err
		}
		return map[string]any{"ok": true, "allocations": allocations}, 200, nil
	}
	return nil, 0, warehouseFailure(405, "Method not allowed.")
}

func warehouseDisks() []map[string]any {
	disks := []map[string]any{}
	seen := map[string]bool{}
	if file, err := os.Open("/proc/mounts"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 3 || !strings.HasPrefix(fields[0], "/dev/") {
				continue
			}
			mount := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\").Replace(fields[1])
			excluded := false
			for _, prefix := range []string{"/proc", "/sys", "/dev", "/run", "/snap", "/boot"} {
				if strings.HasPrefix(mount, prefix) {
					excluded = true
					break
				}
			}
			if excluded || seen[mount] {
				continue
			}
			switch fields[2] {
			case "ext4", "ext3", "ext2", "xfs", "btrfs", "vfat", "ntfs", "fuseblk":
			default:
				continue
			}
			usage := warehouseDiskUsage(mount)
			if usage["total"] == 0 {
				continue
			}
			seen[mount] = true
			disks = append(disks, map[string]any{"device": fields[0], "mountpoint": mount, "fstype": fields[2], "total": usage["total"], "used": usage["used"], "free": usage["free"]})
		}
	}
	if !seen["/"] {
		usage := warehouseDiskUsage("/")
		if usage["total"] > 0 {
			disks = append([]map[string]any{{"device": "root", "mountpoint": "/", "fstype": "unknown", "total": usage["total"], "used": usage["used"], "free": usage["free"]}}, disks...)
		}
	}
	return disks
}
