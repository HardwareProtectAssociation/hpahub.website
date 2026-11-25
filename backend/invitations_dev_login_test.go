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

func TestDevelopmentLoginShortcut(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := initInvitations(app); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	member := core.NewRecord(collection)
	member.SetEmail("admin@hpahub.local")
	member.SetRandomPassword()
	if err := app.Save(member); err != nil {
		t.Fatal(err)
	}

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindInvitationRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	login := func(identity, password, remote string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"email": identity, "password": password})
		request := httptest.NewRequest("POST", "/api/hpahub/login", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = remote
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}

	t.Setenv("HPA_DEV_LOGIN", "1")
	allowed := login("1", "1", "127.0.0.1:12345")
	if allowed.Code != http.StatusOK {
		t.Fatalf("shortcut status = %d: %s", allowed.Code, allowed.Body.String())
	}
	var auth struct {
		Token  string          `json:"token"`
		Record json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(allowed.Body.Bytes(), &auth); err != nil || auth.Token == "" {
		t.Fatalf("shortcut did not return a token: %v %s", err, allowed.Body.String())
	}
	if wrong := login("1", "2", "127.0.0.1:12345"); wrong.Code != http.StatusBadRequest {
		t.Fatalf("wrong shortcut password status = %d, want 400", wrong.Code)
	}
	if remote := login("1", "1", "192.0.2.10:12345"); remote.Code != http.StatusBadRequest {
		t.Fatalf("non-loopback shortcut status = %d, want 400", remote.Code)
	}

	t.Setenv("HPA_DEV_LOGIN", "0")
	if disabled := login("1", "1", "127.0.0.1:12345"); disabled.Code != http.StatusBadRequest {
		t.Fatalf("disabled shortcut status = %d, want 400", disabled.Code)
	}
}
