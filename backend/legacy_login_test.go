package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/crypto/scrypt"
)

func TestLegacyLogin(t *testing.T) {
	app := invitationTestApp(t)
	if _, err := app.DB().NewQuery(`CREATE TABLE IF NOT EXISTS hpahub_legacy_accounts (legacy_id INTEGER PRIMARY KEY, user_id TEXT NOT NULL UNIQUE, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL)`).Execute(); err != nil { t.Fatal(err) }
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil { t.Fatal(err) }
	users.MFA.Enabled = false
	if err := app.Save(users); err != nil { t.Fatal(err) }
	record := core.NewRecord(users)
	record.SetRandomPassword()
	record.Set("role", "member")
	if err := app.Save(record); err != nil { t.Fatal(err) }
	password := strings.Repeat("密码🔐", 20)
	key, err := scrypt.Key([]byte(password), []byte("synthetic-salt"), 32768, 8, 1, 64)
	if err != nil { t.Fatal(err) }
	hash := fmt.Sprintf("scrypt:32768:8:1$synthetic-salt$%x", key)
	if _, err := app.DB().NewQuery("INSERT INTO hpahub_legacy_accounts (legacy_id, user_id, username, password_hash) VALUES (1, {:id}, '旧用户', {:hash})").Bind(dbx.Params{"id": record.Id, "hash": hash}).Execute(); err != nil { t.Fatal(err) }
	router, err := apis.NewRouter(app)
	if err != nil { t.Fatal(err) }
	bindInvitationRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil { t.Fatal(err) }
	login := func(name, password string) *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]string{"email": name, "password": password})
		if err != nil { t.Fatal(err) }
		req := httptest.NewRequest(http.MethodPost, "/api/hpahub/login", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	res := login("旧用户", password)
	var response struct { Token string `json:"token"`; Record struct { ID string `json:"id"` } `json:"record"` }
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil || res.Code != http.StatusOK || response.Token == "" || response.Record.ID != record.Id { t.Fatalf("legacy login: %d %s", res.Code, res.Body.String()) }
	for _, tc := range []struct { name, password string }{{"旧用户", password + "x"}, {"旧用户X", password}} {
		if res := login(tc.name, tc.password); res.Code != http.StatusBadRequest { t.Fatalf("invalid login: %d %s", res.Code, res.Body.String()) }
	}
	legacyLoginSlots <- struct{}{}
	legacyLoginSlots <- struct{}{}
	res = login("旧用户", password)
	<-legacyLoginSlots
	<-legacyLoginSlots
	if res.Code != http.StatusTooManyRequests { t.Fatalf("saturated login: %d %s", res.Code, res.Body.String()) }
	for _, invalid := range []string{"scrypt:16384:8:1$synthetic-salt$" + strings.Repeat("0", 128), "scrypt:32768:8:1$$" + strings.Repeat("0", 128), "scrypt:32768:8:1$salt$xyz", "scrypt:32768:8:1$salt$" + strings.Repeat("z", 128)} {
		if verifyLegacyPassword(password, invalid) { t.Fatalf("accepted invalid hash %q", invalid) }
	}
	users.AuthRule = nil
	if err := app.Save(users); err != nil { t.Fatal(err) }
	if res := login("旧用户", password); res.Code < 400 { t.Fatal("legacy login bypassed auth rule") }
}
