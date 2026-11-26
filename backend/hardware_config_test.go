package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestHardwareConfigRollback(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	user := centerTestUser(t, app, "config@example.com", "member")
	token, _ := user.NewAuthToken()
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindHardwareConfigRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/api/hpahub/me/hardware-config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	if res := call(`{"items":[{"category":"CPU","name":"original"}]}`); res.Code != http.StatusOK {
		t.Fatalf("initial save: %d %s", res.Code, res.Body.String())
	}
	if res := call(`{"items":[{"category":"","name":"invalid"}]}`); res.Code != http.StatusBadRequest {
		t.Fatalf("invalid save: %d", res.Code)
	}
	if _, err := app.DB().NewQuery(`CREATE TRIGGER fail_config BEFORE INSERT ON user_hardware_configs BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Execute(); err != nil {
		t.Fatal(err)
	}
	if res := call(`{"items":[{"category":"CPU","name":"replacement"}]}`); res.Code != http.StatusInternalServerError {
		t.Fatalf("failed save: %d", res.Code)
	}
	rows, err := app.FindRecordsByFilter("user_hardware_configs", "", "", 0, 0)
	if err != nil || len(rows) != 1 || rows[0].GetString("name") != "original" {
		t.Fatalf("config not preserved: %v %v", rows, err)
	}
}
