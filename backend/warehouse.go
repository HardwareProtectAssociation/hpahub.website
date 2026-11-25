package main

import (
	"bufio"
	"context"
	"database/sql"
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

func (s warehouse) projectRecord(name string) (*core.Record, error) {
	if _, err := warehouseName(name); err != nil {
		return nil, err
	}
	record, err := s.app.FindFirstRecordByFilter("warehouse_projects", "name = {:name}", dbx.Params{"name": name})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fs.ErrNotExist
	}
	return record, err
}

func (s warehouse) fileRecord(project, category, name string) (*core.Record, error) {
	if !warehouseCategory(category) {
		return nil, warehouseFailure(400, "非法文件分类")
	}
	if _, err := warehouseName(name); err != nil {
		return nil, err
	}
	p, err := s.projectRecord(project)
	if err != nil {
		return nil, err
	}
	record, err := s.app.FindFirstRecordByFilter("warehouse_files", "project = {:project} && category = {:category} && name = {:name}", dbx.Params{"project": p.Id, "category": category, "name": name})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fs.ErrNotExist
	}
	return record, err
}

func warehouseMetadata(record *core.Record, field string) map[string]any {
	result := map[string]any{}
	_ = record.UnmarshalJSONField(field, &result)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func warehouseModified(record *core.Record) time.Time {
	value, _ := warehouseMetadata(record, "metadata")["modified"].(string)
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", value, time.Local); err == nil {
		return parsed
	}
	return record.GetDateTime("updated").Time()
}

func (s warehouse) size(record *core.Record, field string) (int64, error) {
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return 0, err
	}
	defer storage.Close()
	attributes, err := storage.Attributes(record.BaseFilesPath() + "/" + record.GetString(field))
	if err != nil {
		return 0, err
	}
	return attributes.Size, nil
}

func (s warehouse) temporary(record *core.Record, field string) (string, error) {
	storage, err := s.app.NewFilesystem()
	if err != nil {
		return "", err
	}
	defer storage.Close()
	reader, err := storage.GetReader(record.BaseFilesPath() + "/" + record.GetString(field))
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
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
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

func warehouseTime(t time.Time) string { return t.Local().Format("2006-01-02T15:04:05") }

func (s warehouse) projectFiles(project *core.Record) ([]*core.Record, error) {
	return s.app.FindRecordsByFilter("warehouse_files", "project = {:project}", "name", 0, 0, dbx.Params{"project": project.Id})
}

func (s warehouse) fileEntry(ctx context.Context, project string, record *core.Record) (map[string]any, error) {
	name, category := record.GetString("name"), record.GetString("category")
	size, err := s.size(record, "content")
	if err != nil {
		return nil, err
	}
	entry := map[string]any{"name": name, "category": category, "size": size, "modified": warehouseTime(warehouseModified(record)), "url": warehouseMediaURL("file", project, category, name)}
	if category == "videos" {
		for kind, field := range map[string]string{"thumb": "thumbnail", "web": "web", "preview": "preview"} {
			if record.GetString(field) != "" {
				key := map[string]string{"thumb": "thumb", "web": "web_url", "preview": "preview_url"}[kind]
				entry[key] = warehouseMediaURL(kind, project, category, name)
			}
		}
		metadata := warehouseMetadata(record, "metadata")
		codec, _ := metadata["codec"].(string)
		if codec == "" {
			temporary, err := s.temporary(record, "content")
			if err != nil {
				return nil, err
			}
			codec = warehouseCodec(ctx, temporary)
			os.Remove(temporary)
		}
		entry["codec"] = codec
	}
	return entry, nil
}

func (s warehouse) files(ctx context.Context, project string) (map[string][]map[string]any, error) {
	p, err := s.projectRecord(project)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, warehouseFailure(404, "项目不存在")
	}
	if err != nil {
		return nil, err
	}
	records, err := s.projectFiles(p)
	if err != nil {
		return nil, err
	}
	result := map[string][]map[string]any{}
	for _, category := range warehouseCategories {
		result[category] = []map[string]any{}
	}
	for _, record := range records {
		category := record.GetString("category")
		if !warehouseCategory(category) {
			continue
		}
		entry, err := s.fileEntry(ctx, project, record)
		if err != nil {
			return nil, err
		}
		result[category] = append(result[category], entry)
	}
	return result, nil
}

func (s warehouse) summary(project string) (map[string]any, error) {
	record, err := s.projectRecord(project)
	if err != nil {
		return nil, err
	}
	records, err := s.projectFiles(record)
	if err != nil {
		return nil, err
	}
	meta := warehouseMetadata(record, "metadata")
	counts := map[string]int{}
	for _, category := range warehouseCategories {
		counts[category] = 0
	}
	var size int64
	for _, file := range records {
		fileSize, err := s.size(file, "content")
		if err != nil {
			return nil, err
		}
		counts[file.GetString("category")]++
		size += fileSize
	}
	created := meta["created"]
	if created == nil || created == "" {
		created = warehouseTime(record.GetDateTime("created").Time())
	}
	display := meta["display_name"]
	if display == nil {
		display = project
	}
	return map[string]any{"name": project, "display_name": display, "created": created, "counts": counts, "total_files": len(records), "size_bytes": size, "creator_id": meta["creator_id"], "creator_name": meta["creator_name"]}, nil
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
	if len(parts) == 7 && parts[0] == "api" && parts[1] == "hpahub" && parts[2] == "media" && (r.Method == "GET" || r.Method == "HEAD") {
		s.serveMedia(w, r, parts[3], parts[4], parts[5], parts[6])
		return
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
			entries, err := s.app.FindRecordsByFilter("warehouse_projects", "", "name", 0, 0)
			if err != nil {
				return nil, 0, err
			}
			projects := []map[string]any{}
			for _, entry := range entries {
				name := entry.GetString("name")
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
			if _, err := s.projectRecord(name); err == nil {
				return nil, 0, warehouseFailure(409, "同名项目已存在")
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, 0, err
			}
			collection, err := s.app.FindCollectionByNameOrId("warehouse_projects")
			if err != nil {
				return nil, 0, err
			}
			record := core.NewRecord(collection)
			record.Set("name", name)
			meta := map[string]any{"display_name": name, "created": warehouseTime(time.Now())}
			if identity := warehouseIdentity(r); identity != nil {
				meta["creator_id"], meta["creator_name"] = identity["id"], identity["name"]
			}
			record.Set("metadata", meta)
			if err := s.app.Save(record); err != nil {
				return nil, 0, err
			}
			project, err := s.summary(name)
			return map[string]any{"ok": true, "project": project}, 201, err
		}
		return nil, 0, warehouseFailure(405, "Method not allowed.")
	}
	project := parts[2]
	info, err := s.projectRecord(project)
	if errors.Is(err, fs.ErrNotExist) {
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
				if _, err := s.projectRecord(name); err == nil {
					return nil, 0, warehouseFailure(409, "同名项目已存在")
				} else if !errors.Is(err, fs.ErrNotExist) {
					return nil, 0, err
				}
				meta := warehouseMetadata(info, "metadata")
				meta["display_name"], meta["renamed"] = name, warehouseTime(time.Now())
				info.Set("name", name)
				info.Set("metadata", meta)
				if err := s.app.Save(info); err != nil {
					return nil, 0, err
				}
			}
			summary, err := s.summary(name)
			return map[string]any{"ok": true, "project": summary}, 200, err
		case "DELETE":
			records, err := s.projectFiles(info)
			if err != nil {
				return nil, 0, err
			}
			err = s.app.RunInTransaction(func(app core.App) error {
				for _, record := range records {
					if err := app.Delete(record); err != nil {
						return err
					}
				}
				return app.Delete(info)
			})
			return ok, 200, err
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
	p, err := s.projectRecord(project)
	if err != nil {
		return nil, err
	}
	collection, err := s.app.FindCollectionByNameOrId("warehouse_files")
	if err != nil {
		return nil, err
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
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		for counter := 0; ; counter++ {
			candidate := name
			if counter > 0 {
				candidate = stem + "_" + strconv.Itoa(counter) + ext
			}
			_, err = s.fileRecord(project, category, candidate)
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
		record := core.NewRecord(collection)
		record.Set("project", p.Id)
		record.Set("category", category)
		record.Set("name", name)
		record.Set("metadata", map[string]any{"modified": time.Now().Format(time.RFC3339Nano)})
		if err == nil {
			var content *filesystem.File
			content, err = filesystem.NewFileFromPath(temporary)
			if err == nil {
				record.Set("content", content)
				err = s.app.Save(record)
			}
		}
		os.Remove(temporary)
		if err != nil {
			return nil, err
		}
		entry, err := s.fileEntry(r.Context(), project, record)
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
	record, err := s.fileRecord(project, category, name)
	if errors.Is(err, fs.ErrNotExist) {
		status := 400
		if r.Method == "DELETE" {
			status = 404
		}
		return nil, 0, warehouseFailure(status, "文件不存在")
	}
	if err != nil {
		return nil, 0, err
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
		return map[string]any{"ok": true}, 200, s.app.Delete(record)
	case "PUT", "PATCH":
		data, err := warehouseRequestJSON(r)
		if err != nil {
			return nil, 0, err
		}
		newName, err := warehouseName(warehouseNameField(data))
		if err != nil {
			return nil, 0, err
		}
		if _, err := warehouseName(newName); err != nil {
			return nil, 0, err
		}
		if newName != name {
			if _, err := s.fileRecord(project, category, newName); err == nil {
				return nil, 0, warehouseFailure(400, "同名文件已存在")
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, 0, err
			}
			record.Set("name", newName)
			err = s.app.Save(record)
			if err != nil {
				return nil, 0, err
			}
		}
		entry, err := s.fileEntry(r.Context(), project, record)
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
	field := map[string]string{"thumb": "thumbnail", "web": "web", "preview": "preview"}[kind]
	if category != "videos" || field == "" {
		return "", warehouseFailure(400, "非法媒体类型")
	}
	record, err := s.fileRecord(project, category, name)
	if err != nil {
		return "", err
	}
	if record.GetString(field) != "" {
		return field, nil
	}
	source, err := s.temporary(record, "content")
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
		args = append(args, "-ss", "1", "-i", source, "-vframes", "1", "-vf", "scale=480:-2", "-c:v", "mjpeg", "-q:v", "4", "-f", "image2", temporary)
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
	content, err := filesystem.NewFileFromPath(temporary)
	if err != nil {
		return "", err
	}
	record.Set(field, content)
	if err := s.app.Save(record); err != nil {
		return "", err
	}
	return field, nil
}

func (s warehouse) serveMedia(w http.ResponseWriter, r *http.Request, kind, project, category, name string) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	field := map[string]string{"file": "content", "thumb": "thumbnail", "web": "web", "preview": "preview"}[kind]
	if field == "" || (kind != "file" && category != "videos") {
		http.NotFound(w, r)
		return
	}
	record, err := s.fileRecord(project, category, name)
	if err != nil || record.GetString(field) == "" {
		http.NotFound(w, r)
		return
	}
	storage, err := s.app.NewFilesystem()
	if err != nil {
		http.Error(w, "服务器文件操作失败", 500)
		return
	}
	defer storage.Close()
	size, err := s.size(record, field)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
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
	if err := storage.Serve(w, r, record.BaseFilesPath()+"/"+record.GetString(field), name); err != nil {
		http.NotFound(w, r)
	}
}

func (s warehouse) storage(r *http.Request) (any, int, error) {
	record, err := s.app.FindFirstRecordByFilter("warehouse_settings", "key = 'storage_config'")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}
	config := map[string]any{}
	if record != nil {
		config = warehouseMetadata(record, "value")
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
		entries, err := s.app.FindRecordsByFilter("warehouse_files", "", "", 0, 0)
		if err != nil {
			return nil, 0, err
		}
		for _, file := range entries {
			size, err := s.size(file, "content")
			if err != nil {
				return nil, 0, err
			}
			usage[file.GetString("category")] += size
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
		if record == nil {
			collection, err := s.app.FindCollectionByNameOrId("warehouse_settings")
			if err != nil {
				return nil, 0, err
			}
			record = core.NewRecord(collection)
			record.Set("key", "storage_config")
		}
		record.Set("value", config)
		if err := s.app.Save(record); err != nil {
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
