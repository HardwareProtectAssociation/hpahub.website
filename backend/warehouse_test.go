package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

var warehouseTestApp core.App

func warehouseTestSetup(t *testing.T) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	warehouseTestApp = app
	t.Cleanup(func() { warehouseTestApp = nil; app.Cleanup() })
}

func warehouseTestRequest(t *testing.T, method, path string, body io.Reader, headers map[string]string, status int) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	serveWarehouse(warehouseTestApp, w, r)
	if w.Code != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	var data map[string]any
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
	}
	return w, data
}

func warehouseTestJSON(t *testing.T, method, path string, data any, status int) map[string]any {
	t.Helper()
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	_, result := warehouseTestRequest(t, method, path, bytes.NewReader(body), map[string]string{"Content-Type": "application/json"}, status)
	return result
}

func warehouseTestUpload(t *testing.T, project string, originals, names []string, contents [][]byte, status int) map[string]any {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index, name := range originals {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(contents[index]); err != nil {
			t.Fatal(err)
		}
		if index < len(names) {
			if err := writer.WriteField("names", names[index]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_, data := warehouseTestRequest(t, "POST", "/api/projects/"+url.PathEscape(project)+"/files", &body, map[string]string{"Content-Type": writer.FormDataContentType()}, status)
	return data
}

func warehouseTestFile(t *testing.T, root, project, category, name string, content []byte) string {
	t.Helper()
	directory := filepath.Join(root, "projects", project, category)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWarehouseCRUDAndStorageCompatibility(t *testing.T) {
	warehouseTestSetup(t)
	projectName := "测试 +@项目"
	projectPath := "/api/projects/" + url.PathEscape(projectName)
	_, data := warehouseTestRequest(t, "POST", "/api/projects", strings.NewReader(`{"name":"  测试 +@项目  "}`), map[string]string{"Content-Type": "application/json", "X-User-Id": "trusted-id", "X-User-Name": url.PathEscape("创建 者")}, 201)
	project := data["project"].(map[string]any)
	if project["name"] != projectName || project["creator_id"] != "trusted-id" || project["creator_name"] != "创建 者" {
		t.Fatalf("identity/name: %#v", project)
	}
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": projectName}, 409)
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "other"}, 201)
	_, listing := warehouseTestRequest(t, "GET", "/api/projects", nil, map[string]string{"X-User-Name": url.PathEscape("名称")}, 200)
	if listing["me"].(map[string]any)["id"] != "名称" || len(listing["projects"].([]any)) != 2 {
		t.Fatalf("project list: %#v", listing)
	}

	upload := warehouseTestUpload(t, projectName,
		[]string{"first.bin", "second.bin", `C:\browser\third.txt`, "CON.txt", "photo.PNG", "blob.bin"},
		[]string{"记录 +@.txt", "记录 +@.txt", "", "", "", ""},
		[][]byte{[]byte("first"), []byte("second"), []byte("third"), []byte("reserved"), []byte("image"), []byte("binary")}, 201)
	files := upload["files"].([]any)
	first := files[0].(map[string]any)
	if first["name"] != "记录 +@.txt" || files[1].(map[string]any)["name"] != "记录 +@_1.txt" || files[2].(map[string]any)["name"] != "third.txt" || files[3].(map[string]any)["name"] != "file_3.txt" {
		t.Fatalf("upload names: %#v", files)
	}
	if first["url"] != "/api/hpahub/warehouse/media/file/%E6%B5%8B%E8%AF%95%20%2B%40%E9%A1%B9%E7%9B%AE/docs/%E8%AE%B0%E5%BD%95%20%2B%40.txt" {
		t.Fatalf("legacy URL escaping: %s", first["url"])
	}
	warehouseTestRequest(t, "GET", projectPath, nil, nil, 200)
	_, state := warehouseTestRequest(t, "GET", "/api/projects", nil, nil, 200)
	found := false
	for _, value := range state["projects"].([]any) {
		item := value.(map[string]any)
		if item["name"] == projectName {
			found = true
			if item["total_files"] != float64(6) || item["size_bytes"] != float64(35) || item["counts"].(map[string]any)["docs"] != float64(4) {
				t.Fatalf("counts: %#v", item)
			}
		}
	}
	if !found {
		t.Fatal("uploaded project missing")
	}
	storage := warehouseTestJSON(t, "PUT", "/api/storage", map[string]any{"allocations": map[string]any{"/": 1073741824}}, 200)
	if storage["allocations"].(map[string]any)["/"] != float64(1073741824) {
		t.Fatal(storage)
	}
	if _, err := warehouseTestApp.FindFirstRecordByFilter("warehouse_items", "path = 'storage_config.json'"); err != nil {
		t.Fatal(err)
	}
	_, storage = warehouseTestRequest(t, "GET", "/api/storage", nil, nil, 200)
	if storage["categories"].(map[string]any)["docs"] != float64(24) || len(storage["disks"].([]any)) == 0 {
		t.Fatalf("storage: %#v", storage)
	}

	filePath := projectPath + "/files/docs/" + url.PathEscape("记录 +@.txt")
	warehouseTestJSON(t, "PUT", filePath, map[string]any{"name": "third.txt"}, 400)
	warehouseTestJSON(t, "PATCH", filePath, map[string]any{"name": "renamed.md"}, 200)
	_, detail := warehouseTestRequest(t, "GET", projectPath, nil, nil, 200)
	if len(detail["files"].(map[string]any)["docs"].([]any)) != 4 {
		t.Fatal("rename unexpectedly reclassified file")
	}
	warehouseTestJSON(t, "PATCH", projectPath, map[string]any{"name": "other"}, 409)
	renamed := warehouseTestJSON(t, "PUT", projectPath, map[string]any{"name": "新项目"}, 200)
	if renamed["project"].(map[string]any)["creator_id"] != "trusted-id" {
		t.Fatal("rename lost creator")
	}
	warehouseTestRequest(t, "GET", projectPath, nil, nil, 404)
	warehouseTestRequest(t, "DELETE", "/api/projects/新项目/files/docs/renamed.md", nil, nil, 200)
	warehouseTestRequest(t, "DELETE", "/api/projects/新项目/files/docs/renamed.md", nil, nil, 404)
	warehouseTestRequest(t, "DELETE", "/api/projects/新项目", nil, nil, 200)
	warehouseTestRequest(t, "GET", "/api/projects/新项目", nil, nil, 404)
}

func TestWarehouseMediaIsolationRangeAndHEAD(t *testing.T) {
	root := t.TempDir()
	warehouseTestSetup(t)
	warehouseTestFile(t, root, "one", "docs", "same.html", []byte("first"))
	warehouseTestFile(t, root, "two", "docs", "same.html", []byte("second"))
	warehouseTestFile(t, root, "one", "images", "safe.png", []byte("png-content"))
	info, err := os.Stat(filepath.Join(root, "projects", "one"))
	if err != nil {
		t.Fatal(err)
	}
	created := warehouseTime(warehouseCreated(info))
	if err := importWarehouse(warehouseTestApp, root); err != nil {
		t.Fatal(err)
	}
	_, project := warehouseTestRequest(t, "GET", "/api/projects/one", nil, nil, 200)
	if project["project"].(map[string]any)["created"] != created {
		t.Fatalf("imported project creation time: %#v", project)
	}
	path := "/api/hpahub/media/file/one/docs/same.html"
	w, _ := warehouseTestRequest(t, "GET", path, nil, nil, 200)
	if w.Body.String() != "first" || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("media headers/body: %v %q", w.Header(), w.Body.String())
	}
	disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || params["filename"] != "same.html" {
		t.Fatal("active content served inline")
	}
	w, _ = warehouseTestRequest(t, "GET", path, nil, map[string]string{"Range": "bytes=1-3"}, 206)
	if w.Body.String() != "irs" || w.Header().Get("Content-Range") != "bytes 1-3/5" {
		t.Fatalf("range: %v %q", w.Header(), w.Body.String())
	}
	w, _ = warehouseTestRequest(t, "HEAD", path, nil, nil, 200)
	if w.Body.Len() != 0 || w.Header().Get("Content-Length") != "5" || w.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("HEAD: %v %q", w.Header(), w.Body.String())
	}
	w, _ = warehouseTestRequest(t, "GET", path, nil, map[string]string{"Range": "bytes=100-200"}, 416)
	if w.Header().Get("Content-Range") != "bytes */5" {
		t.Fatal("invalid range missing file size")
	}
	warehouseTestRequest(t, "GET", "/api/hpahub/media/file/missing/docs/same.html", nil, nil, 404)
	warehouseTestRequest(t, "GET", "/api/hpahub/media/file/one/docs/.meta.json", nil, nil, 404)
	w, _ = warehouseTestRequest(t, "GET", "/api/hpahub/media/file/one/images/safe.png", nil, nil, 200)
	if w.Header().Get("Content-Disposition") != "inline" {
		t.Fatal("safe image not inline")
	}
}

func TestWarehouseValidationAndSymlinkWrites(t *testing.T) {
	warehouseTestSetup(t)
	for _, name := range []any{"", ".", "..", ".private", "../escape", `bad\name`, "bad\x00name", "bad\nname", "name.", "bad?name", "CON.txt", "LPT9.log", strings.Repeat("字", 256), 123, nil} {
		warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": name}, 400)
	}
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "safe"}, 201)
	warehouseTestUpload(t, "safe", []string{"ok.txt"}, []string{"../escape"}, [][]byte{[]byte("secret")}, 400)
	warehouseTestUpload(t, "safe", []string{".hidden"}, nil, [][]byte{[]byte("secret")}, 400)
	warehouseTestRequest(t, "GET", "/api/hpahub/media/file/safe/docs/a%2Fb", nil, nil, 404)
	warehouseTestRequest(t, "GET", "/api/hpahub/media/unknown/safe/docs/a.txt", nil, nil, 404)
	warehouseTestRequest(t, "GET", "/api/hpahub/media/file/safe/unknown/a.txt", nil, nil, 404)
}

type warehouseLimitReader struct{}

func (warehouseLimitReader) Read([]byte) (int, error) { return 0, apis.ErrRequestEntityTooLarge }

func TestWarehouseUploadLimit(t *testing.T) {
	warehouseTestSetup(t)
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "safe"}, 201)
	r := httptest.NewRequest("POST", "/api/projects/safe/files", nil)
	r.ContentLength = 500*1024*1024 + 1
	w := httptest.NewRecorder()
	serveWarehouse(warehouseTestApp, w, r)
	if w.Code != 413 {
		t.Fatalf("500 MiB limit: %d", w.Code)
	}
	warehouseTestRequest(t, "POST", "/api/projects/safe/files", warehouseLimitReader{}, map[string]string{"Content-Type": "multipart/form-data; boundary=boundary"}, 413)
	warehouseTestRequest(t, "POST", "/api/projects/safe/files", strings.NewReader("invalid"), map[string]string{"Content-Type": "multipart/form-data; boundary=boundary"}, 400)
}

func TestWarehouseExistingDiskMetadataAndSidecars(t *testing.T) {
	root := t.TempDir()
	warehouseTestSetup(t)
	warehouseTestFile(t, root, "legacy", "videos", "clip.avi", []byte("legacy-video"))
	meta := []byte(`{"display_name":"旧显示名","created":"2024-01-02T03:04:05","creator_id":"old-id","creator_name":"旧用户","unknown":"preserved"}`)
	if err := os.WriteFile(filepath.Join(root, "projects", "legacy", ".meta.json"), meta, 0600); err != nil {
		t.Fatal(err)
	}
	for directory, name := range map[string]string{".thumbs": "clip.avi.jpg", ".web": "clip.avi", ".preview": "clip.avi"} {
		warehouseTestFile(t, root, "legacy", "videos", filepath.Join(directory, name), []byte("cached"))
	}
	if err := importWarehouse(warehouseTestApp, root); err != nil {
		t.Fatal(err)
	}
	_, detail := warehouseTestRequest(t, "GET", "/api/projects/legacy", nil, nil, 200)
	project := detail["project"].(map[string]any)
	if project["display_name"] != "旧显示名" || project["created"] != "2024-01-02T03:04:05" || project["creator_name"] != "旧用户" || project["total_files"] != float64(1) {
		t.Fatal(project)
	}
	files := detail["files"].(map[string]any)["videos"].([]any)
	entry := files[0].(map[string]any)
	if entry["thumb"] == nil || entry["web_url"] == nil || entry["preview_url"] == nil {
		t.Fatal("existing sidecars not discovered")
	}
	warehouseTestJSON(t, "PUT", "/api/projects/legacy/files/videos/clip.avi", map[string]any{"name": "renamed.mov"}, 200)
	for _, kind := range []string{"thumb", "web", "preview"} {
		w, _ := warehouseTestRequest(t, "GET", "/api/hpahub/media/"+kind+"/legacy/videos/renamed.mov", nil, nil, 200)
		if w.Body.String() != "cached" || w.Header().Get("Content-Disposition") != "inline" {
			t.Fatal("sidecar media incompatible")
		}
	}
	warehouseTestRequest(t, "DELETE", "/api/projects/legacy/files/videos/renamed.mov", nil, nil, 200)
	for _, kind := range []string{"thumb", "web", "preview"} {
		warehouseTestRequest(t, "GET", "/api/hpahub/media/"+kind+"/legacy/videos/renamed.mov", nil, nil, 404)
	}
}

func TestWarehouseFFmpegEndToEnd(t *testing.T) {
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s not installed", name)
		}
	}
	warehouseTestSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := filepath.Join(t.TempDir(), "source.avi")
	if output, err := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=red:s=64x64:r=10:d=2", "-c:v", "mpeg4", source).CombinedOutput(); err != nil {
		t.Fatalf("create real video: %v: %s", err, output)
	}
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "video"}, 201)
	warehouseTestUpload(t, "video", []string{"clip.avi"}, nil, [][]byte{content}, 201)
	_, detail := warehouseTestRequest(t, "GET", "/api/projects/video", nil, nil, 200)
	entry := detail["files"].(map[string]any)["videos"].([]any)[0].(map[string]any)
	if entry["codec"] != "mpeg4" || entry["thumb"] == nil || entry["preview_url"] == nil {
		t.Fatalf("ffprobe/thumbnail/preview: %#v", entry)
	}
	warehouseTestRequest(t, "POST", "/api/projects/video/files/videos/clip.avi/transcode", nil, nil, 200)
	media, _ := warehouseTestRequest(t, "GET", "/api/hpahub/media/web/video/videos/clip.avi", nil, nil, 200)
	preview := filepath.Join(t.TempDir(), "web.mp4")
	if err := os.WriteFile(preview, media.Body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if codec := warehouseCodec(context.Background(), preview); codec != "h264" {
		t.Fatalf("web codec: %s", codec)
	}
	w, _ := warehouseTestRequest(t, "GET", "/api/hpahub/media/web/video/videos/clip.avi", nil, map[string]string{"Range": "bytes=0-15"}, 206)
	if w.Body.Len() != 16 || w.Header().Get("Content-Type") != "video/mp4" || w.Header().Get("Content-Disposition") != "inline" {
		t.Fatalf("transcoded byte range: %v", w.Header())
	}
	warehouseTestUpload(t, "video", []string{"broken.avi"}, nil, [][]byte{[]byte("not a video")}, 201)
	warehouseTestRequest(t, "POST", "/api/projects/video/files/videos/broken.avi/transcode", nil, nil, 400)
	warehouseTestRequest(t, "GET", "/api/hpahub/media/web/video/videos/broken.avi", nil, nil, 404)
}

type warehouseBodyError struct{ err error }

func (reader warehouseBodyError) Read([]byte) (int, error) { return 0, reader.err }

func TestWarehouseUploadLimits(t *testing.T) {
	warehouseTestSetup(t)
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "limits"}, 201)
	request := httptest.NewRequest("POST", "/api/projects/limits/files", nil)
	request.ContentLength = warehouseBodyLimit + 1
	response := httptest.NewRecorder()
	serveWarehouse(warehouseTestApp, response, request)
	if response.Code != 413 {
		t.Fatalf("500 MiB ceiling: %d", response.Code)
	}
	request = httptest.NewRequest("POST", "/api/projects/limits/files", warehouseBodyError{apis.ErrRequestEntityTooLarge})
	request.Header.Set("Content-Type", "multipart/form-data; boundary=limit")
	response = httptest.NewRecorder()
	serveWarehouse(warehouseTestApp, response, request)
	if response.Code != 413 {
		t.Fatalf("PocketBase body limit: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest("POST", "/api/projects/limits/files", nil)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=limit")
	request.Body = http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(strings.NewReader(strings.Repeat("x", 64))), 8)
	response = httptest.NewRecorder()
	serveWarehouse(warehouseTestApp, response, request)
	if response.Code != 413 {
		t.Fatalf("streaming body limit: %d %s", response.Code, response.Body.String())
	}
}

func TestWarehouseBackupRestoreImmediatelyAfterDelete(t *testing.T) {
	warehouseTestSetup(t)
	warehouseTestJSON(t, "POST", "/api/projects", map[string]any{"name": "backup"}, 201)
	warehouseTestUpload(t, "backup", []string{"saved.txt"}, nil, [][]byte{[]byte("before backup")}, 201)
	if err := warehouseTestApp.CreateBackup(context.Background(), "warehouse-delete.zip"); err != nil {
		t.Fatal(err)
	}
	warehouseTestRequest(t, "DELETE", "/api/projects/backup/files/docs/saved.txt", nil, nil, 200)

	// Restore into the same isolated PB data directory so any delayed cleanup
	// from the deletion can race the restored files.
	dataDir := warehouseTestApp.DataDir()
	if err := warehouseTestApp.ResetBootstrapState(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "backups" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dataDir, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	warehouseTestExtractBackup(t, filepath.Join(dataDir, "backups", "warehouse-delete.zip"), dataDir)
	if err := warehouseTestApp.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		w, _ := warehouseTestRequest(t, "GET", "/api/hpahub/media/file/backup/docs/saved.txt", nil, nil, 200)
		if w.Body.String() != "before backup" {
			t.Fatalf("restored file content: %q", w.Body.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
}
