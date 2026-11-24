package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGPUAPI(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, private bool) (int, map[string]any, string) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/api/"+path, strings.NewReader(body))
		serveGPU(app, w, r, private)
		var data map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &data)
		return w.Code, data, w.Body.String()
	}
	code, created, _ := call("POST", "gpus", `{"gpu":{"model":"GPU","vram_size":"16","sort_order":2},"games":[{"game":"Game","avg_fps":"3.5"}],"cooling":{"core_temp":"71"},"custom":{"basic":{"hidden":["tgp"],"extra":[{"key":"x","label":"X","value":"v"}]}}}`, true)
	if code != 201 {
		t.Fatalf("create: %d %#v", code, created)
	}
	id := int(created["id"].(float64))
	path := fmt.Sprintf("gpus/%d", id)
	code, detail, _ := call("GET", path, "", true)
	if code != 200 || detail["gpu"].(map[string]any)["vram_size"] != "16" || detail["games"].([]any)[0].(map[string]any)["avg_fps"] != "3.5" {
		t.Fatalf("detail: %d %#v", code, detail)
	}
	code, _, _ = call("POST", fmt.Sprintf("related-media?gpu_id=%d", id), `{"action":"add","path":"private"}`, true)
	if code != 200 {
		t.Fatal(code)
	}
	_, _, public := call("GET", path, "", false)
	if strings.Contains(public, "related_media") || strings.Contains(public, "private") {
		t.Fatalf("private media exposed: %s", public)
	}
	_, _, public = call("GET", "gpus", "", false)
	if strings.Contains(public, "related_media") || strings.Contains(public, "private") {
		t.Fatalf("private list exposed: %s", public)
	}
	_, _, private := call("GET", path, "", true)
	if !strings.Contains(private, "private") {
		t.Fatal("private media missing")
	}
	code, copied, _ := call("POST", path+"/copy", "{}", true)
	if code != 201 {
		t.Fatalf("copy: %d %#v", code, copied)
	}
	copyID := int(copied["id"].(float64))
	_, copyDetail, _ := call("GET", fmt.Sprintf("gpus/%d", copyID), "", true)
	if copyDetail["gpu"].(map[string]any)["model"] != "GPU 副本" || len(copyDetail["games"].([]any)) != 1 {
		t.Fatal(copyDetail)
	}
	code, _, _ = call("POST", "reorder", fmt.Sprintf(`{"ids":[%d,%d]}`, copyID, id), true)
	if code != 200 {
		t.Fatal(code)
	}
	code, _, _ = call("PUT", path, `{"gpu":{"model":"changed"},"games":[{"game":"new"}]}`, true)
	if code != 200 {
		t.Fatal(code)
	}
	_, detail, _ = call("GET", path, "", true)
	if detail["gpu"].(map[string]any)["model"] != "changed" || detail["cooling"] != nil {
		t.Fatal(detail)
	}
	code, _, _ = call("PUT", path, `{"gpu":{"model":"lost"},"games":"invalid"}`, true)
	if code != 500 {
		t.Fatal(code)
	}
	_, detail, _ = call("GET", path, "", true)
	if detail["gpu"].(map[string]any)["model"] != "changed" {
		t.Fatal("failed update was not rolled back")
	}
	code, _, _ = call("POST", fmt.Sprintf("related-media?gpu_id=%d", id), `{"action":"add","path":{"unexpected":true}}`, true)
	if code != 200 {
		t.Fatal(code)
	}
	code, item, _ := call("POST", "items", `{"kind":"cpu","name":"CPU","fields":{"related_media":"secret","x":1},"rows":{"a":[]}}`, true)
	if code != 201 {
		t.Fatal(item)
	}
	itemPath := fmt.Sprintf("items/%d", int(item["id"].(float64)))
	_, _, public = call("GET", itemPath, "", false)
	if strings.Contains(public, "related_media") || strings.Contains(public, "secret") {
		t.Fatalf("nested private field: %s", public)
	}
	code, _, _ = call("POST", "options/games", `{"value":"extra"}`, true)
	if code != 200 {
		t.Fatal(code)
	}
	_, _, options := call("GET", "options/games", "", false)
	if !strings.Contains(options, "extra") || !strings.Contains(options, "new") {
		t.Fatal(options)
	}
	_, stats, _ := call("GET", "stats", "", false)
	if stats["gpus"] != float64(2) || stats["itemsByKind"].(map[string]any)["cpu"] != float64(1) {
		t.Fatal(stats)
	}
	code, _, _ = call("DELETE", path, "", true)
	if code != 200 {
		t.Fatal(code)
	}
	code, _, _ = call("GET", path, "", true)
	if code != 404 {
		t.Fatal(code)
	}
}
