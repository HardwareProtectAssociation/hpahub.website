package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func invitationTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	if err := initInvitations(app); err != nil {
		t.Fatal(err)
	}
	var notMembers int
	if err := app.DB().NewQuery("SELECT count(*) FROM users WHERE role != 'member'").Row(&notMembers); err != nil || notMembers != 0 {
		t.Fatalf("existing users not migrated to member: count=%d err=%v", notMembers, err)
	}
	bindNativeAuthLimit(app)
	return app
}

func TestInvitationAtomicConsumption(t *testing.T) {
	app := invitationTestApp(t)
	now := time.Now()
	item, err := newInvitation(app, now, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Code) != 12 || !strings.HasPrefix(item.Code, "HPA-") || item.Expires-item.Created != 86400 {
		t.Fatalf("invalid invitation: %+v", item)
	}
	if _, err := registerInvitation(app, "bad@example.com", "short", "short", item.Code, now, nil); err == nil {
		t.Fatal("invalid password accepted")
	}
	if _, err := registerInvitation(app, "bad@example.com", "different-password-123", "not-the-same", item.Code, now, nil); err == nil {
		t.Fatal("mismatched password accepted")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registerInvitation(app, fmt.Sprintf("race%d@example.com", i), "valid-password-123", "valid-password-123", item.Code, now, nil)
			if err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent successes = %d", successes.Load())
	}
	var stored invitation
	if err := app.DB().NewQuery("SELECT code, created, expires, used, used_by FROM hpahub_invitations WHERE code = {:code}").Bind(map[string]any{"code": item.Code}).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Used == 0 || stored.UsedBy == "" {
		t.Fatalf("missing consumption: %+v", stored)
	}
	expired, err := newInvitation(app, now.Add(-24*time.Hour), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registerInvitation(app, "expired@example.com", "valid-password-123", "valid-password-123", expired.Code, now, nil); err == nil {
		t.Fatal("expired invitation accepted")
	}
}

func TestInvitationAPIRestrictions(t *testing.T) {
	app := invitationTestApp(t)
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindInvitationRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	if collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil || collection.OAuth2.Enabled {
		t.Fatal("users writes or OAuth2 are not locked")
	}
	if siteRole(nil) != "guest" {
		t.Fatal("anonymous role is not guest")
	}
	unknown := core.NewRecord(collection)
	if siteRole(unknown) != "user" {
		t.Fatal("missing role did not fall back to user")
	}
	unknown.Set("role", "unexpected")
	if siteRole(unknown) != "user" {
		t.Fatal("invalid role did not fall back to user")
	}
	request := func(method, path, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res.Code
	}
	if status := request(http.MethodPost, "/api/collections/users/records", `{"email":"bypass@example.com","password":"valid-password-123","passwordConfirm":"valid-password-123"}`); status < 400 {
		t.Fatalf("native registration returned %d", status)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if status := request(method, "/api/hpahub/invitations", ""); status != http.StatusUnauthorized {
			t.Fatalf("anonymous %s returned %d", method, status)
		}
	}
	if status := request(http.MethodGet, "/api/hpahub/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous me returned %d", status)
	}
	for _, identity := range []struct {
		collection, role string
		status           int
	}{
		{"users", "user", http.StatusForbidden},
		{"users", "member", http.StatusOK},
		{"users", "admin", http.StatusOK},
		{core.CollectionNameSuperusers, "octopus", http.StatusOK},
	} {
		collection, err := app.FindCollectionByNameOrId(identity.collection)
		if err != nil {
			t.Fatal(err)
		}
		record := core.NewRecord(collection)
		record.SetEmail("hpa-role-" + identity.role + "@example.com")
		record.SetPassword("valid-password-123")
		if identity.collection == "users" {
			record.Set("role", identity.role)
		}
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
		token, err := record.NewAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			req := httptest.NewRequest(method, "/api/hpahub/invitations", nil)
			req.Header.Set("Authorization", token)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != identity.status {
				t.Fatalf("%s %s returned %d: %s", identity.role, method, res.Code, res.Body.String())
			}
		}
		req := httptest.NewRequest(http.MethodGet, "/api/hpahub/me", nil)
		req.Header.Set("Authorization", token)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		var me struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &me); err != nil || res.Code != http.StatusOK || me.ID != record.Id || me.Role != identity.role {
			t.Fatalf("invalid me response: %d %s", res.Code, res.Body.String())
		}
		if identity.collection == "users" && identity.role == "admin" {
			req = httptest.NewRequest(http.MethodGet, "/api/backups", nil)
			req.Header.Set("Authorization", token)
			res = httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code < 400 {
				t.Fatal("admin accessed backups")
			}
		}
		if identity.collection == "users" {
			req = httptest.NewRequest(http.MethodPatch, "/api/collections/users/records/"+record.Id, strings.NewReader(`{"role":"admin"}`))
			req.Header.Set("Authorization", token)
			req.Header.Set("Content-Type", "application/json")
			res = httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code < 400 {
				t.Fatalf("%s changed role: %d", identity.role, res.Code)
			}
		}
	}
	for _, collectionName := range []string{"users", core.CollectionNameSuperusers} {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			t.Fatal(err)
		}
		record := core.NewRecord(collection)
		record.SetEmail("shared@example.com")
		if collectionName == "users" {
			record.SetPassword("user-password-123")
		} else {
			record.SetPassword("admin-password-123")
		}
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	if status := request(http.MethodPost, "/api/hpahub/login", `{"email":"shared@example.com","password":"user-password-123"}`); status < 400 {
		t.Fatal("shared email fell back to user")
	}
	if status := request(http.MethodPost, "/api/hpahub/login", `{"email":"shared@example.com","password":"admin-password-123"}`); status != http.StatusOK {
		t.Fatalf("admin login returned %d", status)
	}
	localAdmin, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	localRecord := core.NewRecord(localAdmin)
	localRecord.SetEmail("admin@hpahub.local")
	localRecord.SetPassword("admin123")
	if err := app.Save(localRecord); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HPA_LOCAL_TEST", "")
	if status := request(http.MethodPost, "/api/hpahub/login", `{"email":"admin","password":"admin123"}`); status < 400 {
		t.Fatal("local alias enabled outside local test")
	}
	t.Setenv("HPA_LOCAL_TEST", "1")
	if status := request(http.MethodPost, "/api/hpahub/login", `{"email":"admin","password":"admin123"}`); status != http.StatusOK {
		t.Fatalf("local alias login returned %d", status)
	}
	item, err := newInvitation(app, time.Now(), "")
	if err != nil {
		t.Fatal(err)
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	authRule := users.AuthRule
	users.AuthRule = nil
	if err := app.Save(users); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"email":"rollback@example.com","password":"valid-password-123","passwordConfirm":"valid-password-123","inviteCode":%q,"role":"admin"}`, item.Code)
	if status := request(http.MethodPost, "/api/hpahub/register", body); status < 400 {
		t.Fatal("registration ignored auth rule")
	}
	users.AuthRule = authRule
	if err := app.Save(users); err != nil {
		t.Fatal(err)
	}
	if status := request(http.MethodPost, "/api/hpahub/register", body); status != http.StatusBadRequest {
		t.Fatalf("MFA registration returned %d", status)
	}
	users.MFA.Enabled = false
	if err := app.Save(users); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/hpahub/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("failed registration consumed invitation: %d", res.Code)
	}
	var result struct {
		Token  string `json:"token"`
		Record struct {
			CollectionName string `json:"collectionName"`
		} `json:"record"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || result.Token == "" || result.Record.CollectionName != "users" {
		t.Fatalf("missing authenticated user: %s", res.Body.String())
	}
	registered, err := app.FindAuthRecordByEmail("users", "rollback@example.com")
	if err != nil || registered.GetString("role") != "member" {
		t.Fatalf("registration role was not member: %v", err)
	}
	for range 30 {
		request(http.MethodPost, "/api/hpahub/register", `{}`)
	}
	if status := request(http.MethodPost, "/api/hpahub/register", `{}`); status != http.StatusTooManyRequests {
		t.Fatalf("rate limit returned %d", status)
	}
}
func TestInvitationBatchLimit(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := initInvitations(app); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	member := core.NewRecord(collection)
	member.SetEmail("batch@example.com")
	member.SetPassword("valid-password-123")
	member.Set("role", "member")
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
	bindInvitationRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/hpahub/invitations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := request(`{"count":5}`)
	if response.Code != http.StatusOK {
		t.Fatalf("batch status = %d: %s", response.Code, response.Body.String())
	}
	var batch struct {
		Items []invitation `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &batch); err != nil || len(batch.Items) != 5 {
		t.Fatalf("batch items = %+v, %v", batch.Items, err)
	}
	if response := request(`{"count":6}`); response.Code != http.StatusBadRequest {
		t.Fatalf("count 6 status = %d, want 400", response.Code)
	}
}
