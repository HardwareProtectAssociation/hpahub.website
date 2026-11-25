package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func TestHardwareAPI(t *testing.T) {
	app := invitationTestApp(t)
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindHardwareRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	identity := func(role string) string {
		t.Helper()
		c, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			t.Fatal(err)
		}
		r := core.NewRecord(c)
		r.SetEmail("hardware-" + role + "@example.com")
		r.SetPassword("valid-password-123")
		r.Set("role", role)
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
		token, err := r.NewAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	member, user := identity("member"), identity("user")
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	want := func(res *httptest.ResponseRecorder, status int) {
		t.Helper()
		if res.Code != status {
			t.Fatalf("status=%d want=%d body=%s", res.Code, status, res.Body.String())
		}
	}
	object := func(res *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil {
			t.Fatalf("invalid JSON: %s: %v", res.Body.String(), err)
		}
		return value
	}
	const base = "/api/hpahub/hardware"
	const gpu = `{"kind":"gpu","name":"GPU","brand":"Brand","subtitle":"subtitle","sort_order":2,"data":{"fields":{"model":"GPU","vram_size":"16"},"sections":{"cooling":{"core_temp":"71"}},"rows":{"games":[{"game":"Game","avg_fps":"3.5"}]},"custom":{"basic":{"extra":[{"key":"x","value":"v"}]}}}}`
	want(request("POST", base+"/records", "", gpu), http.StatusUnauthorized)
	want(request("POST", base+"/records", user, gpu), http.StatusForbidden)
	schema := request("GET", base+"/schema", "", "")
	want(schema, http.StatusOK)
	for _, fragment := range []string{`"key":"gpu"`, `"key":"cpu"`, `"key":"device"`, `"name_field":"model"`, `"显卡型号"`, `"游戏测试"`, `"avg_fps"`} {
		if !strings.Contains(schema.Body.String(), fragment) {
			t.Fatalf("schema missing %s: %s", fragment, schema.Body.String())
		}
	}
	want(request("POST", base+"/records", member, `{"kind":"toaster","name":"x","data":{"fields":{}}}`), http.StatusBadRequest)
	created := request("POST", base+"/records", member, gpu)
	want(created, http.StatusCreated)
	id, ok := object(created)["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing id: %s", created.Body.String())
	}
	path := base + "/records/" + id
	detail := request("GET", path, "", "")
	want(detail, http.StatusOK)
	value := object(detail)
	data, ok := value["data"].(map[string]any)
	if !ok || value["id"] != id || value["kind"] != "gpu" || value["name"] != "GPU" || value["brand"] != "Brand" || value["subtitle"] != "subtitle" || value["sort_order"] != float64(2) {
		t.Fatalf("detail: %#v", value)
	}
	fields, ok := data["fields"].(map[string]any)
	if !ok || fields["vram_size"] != "16" || !strings.Contains(detail.Body.String(), `"3.5"`) {
		t.Fatalf("detail data: %#v", data)
	}
	want(request("GET", path+"/media", "", ""), http.StatusUnauthorized)
	want(request("GET", path+"/media", user, ""), http.StatusForbidden)
	want(request("PUT", path+"/media", "", `{"paths":["secret/path"]}`), http.StatusUnauthorized)
	want(request("PUT", path+"/media", member, `{"paths":["secret/path","second"]}`), http.StatusOK)
	media := request("GET", path+"/media", member, "")
	want(media, http.StatusOK)
	paths, ok := object(media)["paths"].([]any)
	if !ok || len(paths) != 2 || paths[0] != "secret/path" || paths[1] != "second" {
		t.Fatalf("media: %s", media.Body.String())
	}
	for _, url := range []string{base + "/records", path} {
		res := request("GET", url, "", "")
		want(res, http.StatusOK)
		if strings.Contains(res.Body.String(), "secret/path") || strings.Contains(res.Body.String(), "related_media") {
			t.Fatalf("public media leak: %s", res.Body.String())
		}
	}
	for _, name := range []string{"hardware", "hardware_media", "hardware_options", "hw_types", "hw_fields", "hw_tables"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("%s collection rules are not private", name)
		}
		want(request("GET", "/api/collections/"+name+"/records", "", ""), http.StatusForbidden)
		want(request("GET", "/api/collections/"+name+"/records", member, ""), http.StatusForbidden)
	}
	want(request("PUT", path, user, gpu), http.StatusForbidden)
	want(request("PUT", path, member, `{"kind":"gpu","name":"bad","data":{"fields":{"nested":{"related_media":"secret"}}}}`), http.StatusBadRequest)
	want(request("PUT", path, member, `{"kind":"cpu","name":"wrong","data":{}}`), http.StatusBadRequest)
	detail = request("GET", path, "", "")
	want(detail, http.StatusOK)
	if object(detail)["name"] != "GPU" {
		t.Fatal("failed update changed record")
	}
	const edited = `{"kind":"gpu","name":"changed","brand":"New","subtitle":"","sort_order":3,"data":{"fields":{"model":"changed"},"rows":{"games":[{"game":"new"}]}}}`
	want(request("PUT", path, member, edited), http.StatusOK)
	detail = request("GET", path, "", "")
	want(detail, http.StatusOK)
	if object(detail)["name"] != "changed" || !strings.Contains(detail.Body.String(), `"new"`) || strings.Contains(detail.Body.String(), `"cooling"`) {
		t.Fatalf("edited detail: %s", detail.Body.String())
	}
	media = request("GET", path+"/media", member, "")
	want(media, http.StatusOK)
	if !strings.Contains(media.Body.String(), "secret/path") {
		t.Fatalf("editing removed media: %s", media.Body.String())
	}
	copyResult := request("POST", path+"/copy", member, `{}`)
	want(copyResult, http.StatusCreated)
	copyID, ok := object(copyResult)["id"].(string)
	if !ok || copyID == "" || copyID == id {
		t.Fatalf("copy id: %s", copyResult.Body.String())
	}
	copyPath := base + "/records/" + copyID
	copyDetail := request("GET", copyPath, "", "")
	want(copyDetail, http.StatusOK)
	if object(copyDetail)["name"] != "changed 副本" || !strings.Contains(copyDetail.Body.String(), `"new"`) {
		t.Fatalf("copy: %s", copyDetail.Body.String())
	}
	want(request("POST", base+"/reorder", member, `{"ids":["`+copyID+`","`+id+`"]}`), http.StatusOK)
	list := request("GET", base+"/records", "", "")
	want(list, http.StatusOK)
	var records []map[string]any
	if err := json.Unmarshal(list.Body.Bytes(), &records); err != nil || len(records) != 2 || records[0]["id"] != copyID || records[1]["id"] != id {
		t.Fatalf("order: %s err=%v", list.Body.String(), err)
	}
	want(request("POST", base+"/reorder", member, `{"ids":["`+id+`","missing"]}`), http.StatusBadRequest)
	list = request("GET", base+"/records", "", "")
	if err := json.Unmarshal(list.Body.Bytes(), &records); err != nil || len(records) != 2 || records[0]["id"] != copyID {
		t.Fatalf("failed reorder changed order: %s err=%v", list.Body.String(), err)
	}
	want(request("POST", base+"/options/games", user, `{"value":"extra"}`), http.StatusForbidden)
	want(request("POST", base+"/options/games", member, `{"value":"extra"}`), http.StatusOK)
	want(request("POST", base+"/options/games", member, `{"value":"extra"}`), http.StatusOK)
	options := request("GET", base+"/options/games", "", "")
	want(options, http.StatusOK)
	var games []string
	if err := json.Unmarshal(options.Body.Bytes(), &games); err != nil || len(games) != 2 || !strings.Contains(options.Body.String(), `"extra"`) || !strings.Contains(options.Body.String(), `"new"`) {
		t.Fatalf("options: %s err=%v", options.Body.String(), err)
	}
	cpuResult := request("POST", base+"/records", member, `{"kind":"cpu","name":"CPU","brand":"Brand","subtitle":"desktop","sort_order":4,"data":{"fields":{"cores":8},"rows":{"bench":[{"score":42}]}}}`)
	want(cpuResult, http.StatusCreated)
	cpuID, ok := object(cpuResult)["id"].(string)
	if !ok || cpuID == "" {
		t.Fatalf("CPU id: %s", cpuResult.Body.String())
	}
	cpu := request("GET", base+"/records/"+cpuID, "", "")
	want(cpu, http.StatusOK)
	if !strings.Contains(cpu.Body.String(), `"cores":8`) || !strings.Contains(cpu.Body.String(), `"score":42`) {
		t.Fatalf("CPU data: %s", cpu.Body.String())
	}
	deviceResult := request("POST", base+"/records", member, `{"kind":"device","name":"Device","data":{"fields":{"color":"black"},"rows":{"ports":[]}}}`)
	want(deviceResult, http.StatusCreated)
	deviceID, ok := object(deviceResult)["id"].(string)
	if !ok || deviceID == "" {
		t.Fatalf("device id: %s", deviceResult.Body.String())
	}
	device := request("GET", base+"/records/"+deviceID, "", "")
	want(device, http.StatusOK)
	if !strings.Contains(device.Body.String(), `"color":"black"`) || !strings.Contains(device.Body.String(), `"ports":[]`) {
		t.Fatalf("device data: %s", device.Body.String())
	}
	want(request("DELETE", path, user, ""), http.StatusForbidden)
	want(request("DELETE", path, member, ""), http.StatusOK)
	want(request("GET", path, "", ""), http.StatusNotFound)
	want(request("GET", path+"/media", member, ""), http.StatusNotFound)
	var mediaCount int
	if err := app.DB().NewQuery("SELECT count(*) FROM hardware_media WHERE hardware = {:id}").Bind(map[string]any{"id": id}).Row(&mediaCount); err != nil || mediaCount != 0 {
		t.Fatalf("media not deleted: count=%d err=%v", mediaCount, err)
	}
}
