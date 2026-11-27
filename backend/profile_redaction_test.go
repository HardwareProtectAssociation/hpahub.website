package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	pbrouter "github.com/pocketbase/pocketbase/tools/router"
)

func TestPublicProfileIndividualRedaction(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	owner := publicProjectTestUser(t, app, "redact-owner@example.com", "绝密负责人")
	owner.Set("profilePublic", true)
	owner.Set("bio", "我的保密简介")
	owner.Set("current_status", "busy")
	centerTestSave(t, app, owner)
	other := publicProjectTestUser(t, app, "redact-member@example.com", "绝密成员")
	other.Set("profilePublic", true)
	centerTestSave(t, app, other)
	project := centerTestProject(t, app, owner, "公开项目甲", "ongoing")
	project.Set("summary", "项目公开介绍")
	centerTestSave(t, app, project)
	otherProject := centerTestProject(t, app, owner, "公开项目乙", "ongoing")
	membership := centerTestRecord(t, app, "project_members")
	membership.Set("project_id", project.Id)
	membership.Set("user_id", other.Id)
	membership.Set("status", "active")
	membership.Set("joined_at", time.Now())
	membership.Set("duties", "绝密职责")
	centerTestSave(t, app, membership)
	card := centerTestRecord(t, app, "member_cards")
	card.Set("owner", owner.Id)
	card.Set("title", "保密卡片")
	card.Set("text", "保密卡片正文")
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imageFile, err := filesystem.NewFileFromBytes(imageData.Bytes(), "secret.png")
	if err != nil {
		t.Fatal(err)
	}
	card.Set("images", imageFile)
	centerTestSave(t, app, card)
	otherCard := centerTestRecord(t, app, "member_cards")
	otherCard.Set("owner", owner.Id)
	otherCard.Set("title", "公开卡片")
	centerTestSave(t, app, otherCard)
	skill := centerTestSkill(t, app)
	skillRow := centerTestRecord(t, app, "user_skills")
	skillRow.Set("user_id", owner.Id)
	skillRow.Set("skill_id", skill.Id)
	skillRow.Set("ability_note", "绝密能力")
	skillRow.Set("links", []string{"https://secret.invalid/hidden"})
	skillRow.Set("featured", true)
	skillImage, err := filesystem.NewFileFromBytes(imageData.Bytes(), "secret-skill.png")
	if err != nil {
		t.Fatal(err)
	}
	skillRow.Set("images", skillImage)
	centerTestSave(t, app, skillRow)
	identity := centerTestIdentity(t, app)
	identityRow := centerTestRecord(t, app, "user_identities")
	identityRow.Set("user_id", owner.Id)
	identityRow.Set("identity_id", identity.Id)
	identityRow.Set("featured", true)
	centerTestSave(t, app, identityRow)
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	event := &core.ServeEvent{App: app, Router: router}
	bindPublicRoutes(event)
	bindPublicProjectRoutes(event)
	bindPersonalCenterRoutes(event)
	bindHomeRoutes(event)
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	token, err := owner.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := other.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, auth, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Body = &pbrouter.RereadableReadCloser{ReadCloser: req.Body}
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	put := func(auth string, hidden []string) {
		body, _ := json.Marshal(map[string]any{"profilePublic": true, "hiddenSections": hidden})
		r := call("PUT", "/api/hpahub/me/profile-visibility", auth, string(body))
		if r.Code != 200 {
			t.Fatalf("save: %d %s", r.Code, r.Body.String())
		}
	}
	put(token, []string{"bio", "status", "card:" + card.Id, "skill:" + skillRow.Id, "identity:" + identity.Id, "project:" + project.Id})
	put(otherToken, []string{"project:" + project.Id})
	res := call("GET", "/api/hpahub/members/"+owner.Id, "", "")
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	for _, secret := range []string{"我的保密简介", "busy", "保密卡片", "保密卡片正文", card.Id, "绝密能力", "https://secret.invalid/hidden", skillRow.Id, identity.GetString("name"), identity.Id, "公开项目甲", "项目公开介绍", project.Id} {
		if strings.Contains(res.Body.String(), secret) {
			t.Fatalf("public profile leaked %q: %s", secret, res.Body.String())
		}
	}
	for _, visible := range []string{"公开卡片", "公开项目乙", otherProject.Id, "0x"} {
		if !strings.Contains(res.Body.String(), visible) {
			t.Fatalf("missing %q: %s", visible, res.Body.String())
		}
	}
	var data struct {
		Bio   string `json:"bio"`
		Cards []struct {
			Redacted   bool   `json:"redacted"`
			Title      string `json:"title"`
			Text       string `json:"text"`
			ImageCount int    `json:"imageCount"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Bio != maskedText("我的保密简介") {
		t.Fatalf("wrong mask: %q", data.Bio)
	}
	found := false
	for _, item := range data.Cards {
		if item.Redacted {
			found = true
			if item.Title != maskedText("保密卡片") || item.Text != maskedText("保密卡片正文") || item.ImageCount != 1 {
				t.Fatalf("incorrect card mask: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("missing redacted card")
	}
	imagePath := "/api/hpahub/members/" + owner.Id + "/cards/" + card.Id + "/images/" + card.GetStringSlice("images")[0]
	if r := call("GET", imagePath, "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("hidden card image: %d", r.Code)
	}
	skillPath := "/api/hpahub/skills/" + skillRow.Id + "/images/" + skillRow.GetStringSlice("images")[0]
	if r := call("GET", skillPath, "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("hidden skill image: %d", r.Code)
	}
	if r := call("GET", skillPath, token, ""); r.Code != http.StatusOK {
		t.Fatalf("owner cannot view skill image: %d", r.Code)
	}
	for _, path := range []string{"/api/hpahub/projects", "/api/hpahub/projects/" + project.Id, "/api/hpahub/home"} {
		r := call("GET", path, "", "")
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
		}
		if !strings.Contains(r.Body.String(), "公开项目甲") {
			t.Fatalf("project missing from %s: %s", path, r.Body.String())
		}
		if strings.Contains(path, project.Id) {
			for _, secret := range []string{"绝密负责人", "绝密成员", "绝密职责", owner.Id, other.Id} {
				if strings.Contains(r.Body.String(), secret) {
					t.Fatalf("%s leaked %q: %s", path, secret, r.Body.String())
				}
			}
		} else {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(r.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			key := "items"
			if path == "/api/hpahub/home" {
				key = "popularProjects"
			}
			var rows []publicProject
			if err := json.Unmarshal(payload[key], &rows); err != nil {
				t.Fatal(err)
			}
			foundProject := false
			for _, item := range rows {
				if item.ID == project.Id {
					foundProject = true
					if !item.OwnerRedacted || item.OwnerID != "" || item.OwnerName != maskedText("绝密负责人") {
						t.Fatalf("%s leaked project owner: %+v", path, item)
					}
				}
			}
			if !foundProject {
				t.Fatalf("missing project in %s", path)
			}
		}
	}
	put(token, []string{})
	restored := call("GET", "/api/hpahub/members/"+owner.Id, "", "")
	for _, text := range []string{"保密卡片", "绝密能力", "公开项目甲"} {
		if !strings.Contains(restored.Body.String(), text) {
			t.Fatalf("restore missing %q: %s", text, restored.Body.String())
		}
	}
}
