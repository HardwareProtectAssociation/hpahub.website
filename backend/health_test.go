package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDailyHealthCheck(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	member := centerTestUser(t, app, "health@example.com", "member")
	member.Set("daily_health_enabled", true)
	if err := app.Save(member); err != nil {
		t.Fatal(err)
	}
	token, err := member.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindHealthRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := request("GET", "/api/hpahub/me/health", "")
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d: %s", response.Code, response.Body.String())
	}
	var state struct {
		Enabled        bool `json:"enabled"`
		CheckedInToday bool `json:"checkedInToday"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil || !state.Enabled || state.CheckedInToday {
		t.Fatalf("unexpected health state: %+v, %v", state, err)
	}
	response = request("POST", "/api/hpahub/me/health/checkin", `{"mood":"good","rested":true,"exercised":false,"ateOnTime":true,"note":"还不错"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("checkin status = %d: %s", response.Code, response.Body.String())
	}
	response = request("GET", "/api/hpahub/me/health", "")
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil || !state.CheckedInToday {
		t.Fatalf("checkin not recorded: %+v, %v", state, err)
	}
	if response := request("POST", "/api/hpahub/me/health/checkin", `{"mood":"unknown"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid mood status = %d, want 400", response.Code)
	}
	response = request("PATCH", "/api/hpahub/me/health/settings", `{"enabled":false}`)
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d: %s", response.Code, response.Body.String())
	}
	response = request("GET", "/api/hpahub/me/health", "")
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil || state.Enabled {
		t.Fatalf("settings not saved: %+v, %v", state, err)
	}
}
