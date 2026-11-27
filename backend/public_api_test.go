package main

import (
	"bytes"
	"encoding/json"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	pbrouter "github.com/pocketbase/pocketbase/tools/router"
)

func TestPublicProfilesCardsAndShowcase(t *testing.T) {
	app := invitationTestApp(t)
	router, err := apis.NewRouter(app)
	if err != nil { t.Fatal(err) }
	bindPublicRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil { t.Fatal(err) }
	identity := func(collectionName, role, email string) (*core.Record, string) {
		t.Helper()
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil { t.Fatal(err) }
		record := core.NewRecord(collection)
		record.SetEmail(email + "@example.com")
		record.SetPassword("valid-password-123")
		if collectionName == "users" { record.Set("role", role); record.Set("name", role) }
		if err := app.Save(record); err != nil { t.Fatal(err) }
		token, err := record.NewAuthToken()
		if err != nil { t.Fatal(err) }
		return record, token
	}
	member, memberToken := identity("users", "member", "member")
	_, otherToken := identity("users", "member", "other")
	_, userToken := identity("users", "user", "user")
	_, adminToken := identity("users", "admin", "admin")
	_, superToken := identity(core.CollectionNameSuperusers, "octopus", "octopus")

	request := func(method, path, token, contentType string, body io.Reader) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, body)
		if contentType == "application/json" { req.Body = &pbrouter.RereadableReadCloser{ReadCloser: req.Body} }
		if token != "" { req.Header.Set("Authorization", token) }
		if contentType != "" { req.Header.Set("Content-Type", contentType) }
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	jsonRequest := func(method, path, token, body string) *httptest.ResponseRecorder {
		return request(method, path, token, "application/json", strings.NewReader(body))
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil { t.Fatal(err) }
	multipartRequest := func(method, path, token string, fields map[string][]string, images int) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, values := range fields { for _, value := range values { if err := writer.WriteField(key, value); err != nil { t.Fatal(err) } } }
		for range images {
			part, err := writer.CreateFormFile("images", "picture.png")
			if err != nil { t.Fatal(err) }
			if _, err := part.Write(pngData.Bytes()); err != nil { t.Fatal(err) }
		}
		if err := writer.Close(); err != nil { t.Fatal(err) }
		return request(method, path, token, writer.FormDataContentType(), &body)
	}
	want := func(res *httptest.ResponseRecorder, status int) {
		t.Helper()
		if res.Code != status { t.Fatalf("status = %d, want %d: %s", res.Code, status, res.Body.String()) }
	}
	noCache := func(res *httptest.ResponseRecorder) {
		t.Helper()
		if !strings.Contains(strings.ToLower(res.Header().Get("Cache-Control")), "no-store") { t.Fatalf("missing no-store: %v", res.Header()) }
	}
	cacheFile := func(res *httptest.ResponseRecorder) {
		t.Helper()
		if res.Header().Get("Cache-Control") != "private, no-cache" || res.Header().Get("ETag") == "" { t.Fatalf("invalid file cache headers: %v", res.Header()) }
	}
	object := func(res *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var result map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil { t.Fatalf("invalid JSON: %s: %v", res.Body.String(), err) }
		return result
	}

	const base = "/api/hpahub"
	for _, path := range []string{"/me/profile", "/me/cards", "/manage/showcase"} {
		want(jsonRequest("GET", base+path, "", ""), http.StatusUnauthorized)
		want(jsonRequest("GET", base+path, userToken, ""), http.StatusForbidden)
	}
	want(jsonRequest("GET", base+"/manage/showcase", memberToken, ""), http.StatusForbidden)
	want(jsonRequest("GET", base+"/manage/showcase", adminToken, ""), http.StatusOK)
	want(jsonRequest("GET", base+"/manage/showcase", superToken, ""), http.StatusOK)
	want(jsonRequest("GET", base+"/me/profile", superToken, ""), http.StatusForbidden)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"role":"admin"}`), http.StatusBadRequest)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"name":"","profilePublic":true}`), http.StatusBadRequest)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"name":"Member","bio":"private biography","profilePublic":false}`), http.StatusOK)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"bio":"`+strings.Repeat("x", 601)+`"}`), http.StatusBadRequest)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"name":"`+strings.Repeat("x", 81)+`"}`), http.StatusBadRequest)
	var avatarBody bytes.Buffer
	avatarWriter := multipart.NewWriter(&avatarBody)
	avatarPart, err := avatarWriter.CreateFormFile("avatar", "avatar.png")
	if err != nil { t.Fatal(err) }
	if _, err := avatarPart.Write(pngData.Bytes()); err != nil { t.Fatal(err) }
	if err := avatarWriter.Close(); err != nil { t.Fatal(err) }
	want(request("PATCH", base+"/me/profile", memberToken, avatarWriter.FormDataContentType(), &avatarBody), http.StatusOK)
	avatar := base+"/members/"+member.Id+"/avatar"
	want(jsonRequest("GET", avatar, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/me/profile/avatar", memberToken, ""), http.StatusOK)
	want(jsonRequest("GET", base+"/me/profile/avatar", otherToken, ""), http.StatusNotFound)
	want(multipartRequest("POST", base+"/me/cards", memberToken, map[string][]string{"title": {""}}, 0), http.StatusBadRequest)
	want(multipartRequest("POST", base+"/me/cards", memberToken, map[string][]string{"title": {strings.Repeat("x", 81)}}, 0), http.StatusBadRequest)
	want(multipartRequest("POST", base+"/me/cards", memberToken, map[string][]string{"title": {"Too many images"}}, 5), http.StatusBadRequest)
	want(multipartRequest("POST", base+"/me/cards", memberToken, map[string][]string{"title": {"Card"}, "text": {strings.Repeat("x", 281)}}, 0), http.StatusBadRequest)

	card := multipartRequest("POST", base+"/me/cards", memberToken, map[string][]string{"title": {"Card"}, "text": {"Card text"}}, 2)
	want(card, http.StatusOK)
	cardObject := object(card)
	cardID, _ := cardObject["id"].(string)
	if cardID == "" { t.Fatalf("missing card id: %s", card.Body.String()) }
	storedCard, err := app.FindRecordById("member_cards", cardID)
	if err != nil { t.Fatal(err) }
	cardImages := storedCard.GetStringSlice("images")
	if len(cardImages) != 2 { t.Fatalf("card images = %v", cardImages) }
	want(multipartRequest("PATCH", base+"/me/cards/"+cardID, memberToken, map[string][]string{"keepImages": {"not-owned.png"}}, 0), http.StatusBadRequest)
	cardImage := base+"/members/"+member.Id+"/cards/"+cardID+"/images/"+cardImages[0]
	want(jsonRequest("GET", cardImage, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/me/cards/"+cardID+"/images/"+cardImages[0], otherToken, ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/me/cards/"+cardID+"/images/"+cardImages[0], memberToken, ""), http.StatusOK)
	if native := jsonRequest("GET", "/api/files/member_cards/"+cardID+"/"+cardImages[0], "", ""); native.Code < 400 { t.Fatalf("native file bypassed privacy: %d", native.Code) }
	want(multipartRequest("PATCH", base+"/me/cards/"+cardID, otherToken, map[string][]string{"title": {"Stolen"}}, 0), http.StatusNotFound)
	want(jsonRequest("DELETE", base+"/me/cards/"+cardID, otherToken, ""), http.StatusNotFound)

	listing := jsonRequest("GET", base+"/members", "", "")
	want(listing, http.StatusOK)
	noCache(listing)
	if strings.Contains(listing.Body.String(), member.Id) || strings.Contains(listing.Body.String(), "private biography") || !strings.Contains(listing.Body.String(), "匿名成员") { t.Fatalf("private member leaked: %s", listing.Body.String()) }
	want(jsonRequest("GET", base+"/members/"+member.Id, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/members/unknown", "", ""), http.StatusNotFound)
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"profilePublic":true}`), http.StatusOK)
	profile := jsonRequest("GET", base+"/members/"+member.Id, "", "")
	want(profile, http.StatusOK)
	noCache(profile)
	if !strings.Contains(profile.Body.String(), "private biography") || !strings.Contains(profile.Body.String(), cardID) { t.Fatalf("missing public profile content: %s", profile.Body.String()) }

	visibility := base + "/me/profile-visibility"
	_, linkedSuperToken := identity(core.CollectionNameSuperusers, "octopus", "member")
	want(jsonRequest("GET", visibility, "", ""), http.StatusUnauthorized)
	want(jsonRequest("GET", visibility, superToken, ""), http.StatusForbidden)
	want(jsonRequest("GET", visibility, linkedSuperToken, ""), http.StatusOK)
	want(jsonRequest("GET", base+"/me/cards", superToken, ""), http.StatusForbidden)
	want(jsonRequest("GET", base+"/me/cards", linkedSuperToken, ""), http.StatusOK)
	linkedCard := multipartRequest("POST", base+"/me/cards", linkedSuperToken, map[string][]string{"title": {"Admin card"}}, 0)
	want(linkedCard, http.StatusOK)
	linkedCardID, _ := object(linkedCard)["id"].(string)
	if linkedCardID == "" { t.Fatalf("missing admin card id: %s", linkedCard.Body.String()) }
	storedLinked, err := app.FindRecordById("member_cards", linkedCardID)
	if err != nil || storedLinked.GetString("owner") != member.Id { t.Fatalf("admin card owner mismatch: %v", err) }
	want(multipartRequest("PATCH", base+"/me/cards/"+linkedCardID, superToken, map[string][]string{"title": {"No owner"}}, 0), http.StatusForbidden)
	want(multipartRequest("PATCH", base+"/me/cards/"+linkedCardID, linkedSuperToken, map[string][]string{"title": {"Admin card 2"}}, 0), http.StatusOK)
	want(jsonRequest("DELETE", base+"/me/cards/"+linkedCardID, linkedSuperToken, ""), http.StatusNoContent)
	want(jsonRequest("PUT", visibility, memberToken, `{"profilePublic":true,"hiddenSections":["unknown"]}`), http.StatusBadRequest)
	want(jsonRequest("PUT", visibility, memberToken, `{"profilePublic":true,"hiddenSections":[]} {}`), http.StatusBadRequest)
	want(jsonRequest("PUT", visibility, memberToken, `{"profilePublic":true,"hiddenSections":["bio","avatar","status","cards","identities","projects","career","skills"]}`), http.StatusOK)
	hidden := jsonRequest("GET", base+"/members/"+member.Id, "", "")
	want(hidden, http.StatusOK)
	if strings.Contains(hidden.Body.String(), "private biography") || strings.Contains(hidden.Body.String(), cardID) { t.Fatalf("hidden data leaked: %s", hidden.Body.String()) }
	want(jsonRequest("GET", avatar, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", cardImage, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/me/profile/avatar", memberToken, ""), http.StatusOK)
	if !strings.Contains(jsonRequest("GET", base+"/me/cards", memberToken, "").Body.String(), cardID) { t.Fatal("hidden card was lost") }
	want(jsonRequest("PUT", visibility, memberToken, `{"profilePublic":true,"hiddenSections":[]}`), http.StatusOK)
	restored := jsonRequest("GET", base+"/members/"+member.Id, "", "")
	if !strings.Contains(restored.Body.String(), "private biography") || !strings.Contains(restored.Body.String(), cardID) { t.Fatalf("content not restored: %s", restored.Body.String()) }

	avatarResponse := jsonRequest("GET", avatar, "", "")
	want(avatarResponse, http.StatusOK)
	cacheFile(avatarResponse)
	conditional := func(path, token, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if token != "" { req.Header.Set("Authorization", token) }
		req.Header.Set("If-None-Match", etag)
		res := httptest.NewRecorder(); mux.ServeHTTP(res, req); return res
	}
	oldETag := avatarResponse.Header().Get("ETag")
	if res := conditional(avatar, "", oldETag); res.Code != http.StatusNotModified || res.Body.Len() != 0 { t.Fatalf("conditional avatar: %d, %d bytes", res.Code, res.Body.Len()) }
	want(conditional(base+"/me/profile/avatar", otherToken, oldETag), http.StatusNotFound)

	if !bytes.Equal(avatarResponse.Body.Bytes(), pngData.Bytes()) { t.Fatal("avatar contents differ") }
	thumb, err := base64.StdEncoding.DecodeString("UklGRoIAAABXRUJQVlA4IHYAAADQBwCdASqQAJAAPpFIoUylpCMiIGgAsBIJaW7hdVEHHAAAO7NZHVUmyYh1VJsmIdVSbJiHVUmyYh1VJsmIdVSbJiHVUmyCAAD+/r+uf/4DdfFtMv//c4H/c4H/c4H8bbryv8ghV+KyxvbhAHhyu2Hp5iAAAAAA")
	if err != nil { t.Fatal(err) }
	uploadAvatar := func(includeAvatar, includeThumb bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if includeAvatar { part, err := writer.CreateFormFile("avatar", "avatar.png"); if err != nil { t.Fatal(err) }; if _, err := part.Write(pngData.Bytes()); err != nil { t.Fatal(err) } }
		if includeThumb { part, err := writer.CreateFormFile("avatarThumb", "avatar.webp"); if err != nil { t.Fatal(err) }; if _, err := part.Write(thumb); err != nil { t.Fatal(err) } }
		if err := writer.Close(); err != nil { t.Fatal(err) }
		return request("PATCH", base+"/me/profile", memberToken, writer.FormDataContentType(), &body)
	}
	want(uploadAvatar(false, true), http.StatusBadRequest)
	want(uploadAvatar(true, true), http.StatusOK)
	want(conditional(avatar, "", oldETag), http.StatusOK)
	if got := jsonRequest("GET", avatar, "", ""); !bytes.Equal(got.Body.Bytes(), thumb) { t.Fatal("avatar thumbnail contents differ") }
	storedMember, err := app.FindRecordById("users", member.Id); if err != nil { t.Fatal(err) }
	if storedMember.GetString("avatar") == "" || storedMember.GetString("avatar_thumb") == "" { t.Fatal("avatar files not stored") }
	want(uploadAvatar(true, false), http.StatusOK)
	if got := jsonRequest("GET", avatar, "", ""); !bytes.Equal(got.Body.Bytes(), pngData.Bytes()) { t.Fatal("old thumbnail was not cleared") }

	for _, filename := range cardImages {
		imageResponse := jsonRequest("GET", base+"/members/"+member.Id+"/cards/"+cardID+"/images/"+filename, "", "")
		want(imageResponse, http.StatusOK)
		cacheFile(imageResponse)
		if !bytes.Equal(imageResponse.Body.Bytes(), pngData.Bytes()) { t.Fatal("card image contents differ") }
	}
	want(jsonRequest("GET", base+"/members/"+member.Id+"/cards/"+cardID+"/images/not-owned.png", "", ""), http.StatusNotFound)

	project := multipartRequest("POST", base+"/manage/showcase", adminToken, map[string][]string{"title": {"Project"}, "text": {"Project text"}, "published": {"false"}}, 2)
	want(project, http.StatusOK)
	projectID, _ := object(project)["id"].(string)
	if projectID == "" { t.Fatalf("missing project id: %s", project.Body.String()) }
	storedProject, err := app.FindRecordById("showcase_projects", projectID)
	if err != nil { t.Fatal(err) }
	projectImages := storedProject.GetStringSlice("images")
	if len(projectImages) != 2 { t.Fatalf("project images = %v", projectImages) }
	projectImage := base+"/showcase/"+projectID+"/images/"+projectImages[0]
	want(jsonRequest("GET", projectImage, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", base+"/manage/showcase/"+projectID+"/images/"+projectImages[0], memberToken, ""), http.StatusForbidden)
	want(jsonRequest("GET", base+"/manage/showcase/"+projectID+"/images/"+projectImages[0], adminToken, ""), http.StatusOK)
	if native := jsonRequest("GET", "/api/files/showcase_projects/"+projectID+"/"+projectImages[0], "", ""); native.Code < 400 { t.Fatalf("native file bypassed privacy: %d", native.Code) }
	want(jsonRequest("GET", base+"/showcase/"+projectID, "", ""), http.StatusNotFound)
	want(multipartRequest("PATCH", base+"/manage/showcase/"+projectID, memberToken, map[string][]string{"title": {"Denied"}}, 0), http.StatusForbidden)
	want(multipartRequest("PATCH", base+"/manage/showcase/"+projectID, adminToken, map[string][]string{"title": {"Project"}, "text": {"Project text"}, "published": {"true"}, "keepImages": projectImages}, 0), http.StatusOK)
	publicProject := jsonRequest("GET", base+"/showcase/"+projectID, "", "")
	want(publicProject, http.StatusOK)
	noCache(publicProject)
	imageResponse := jsonRequest("GET", projectImage, "", "")
	want(imageResponse, http.StatusOK)
	cacheFile(imageResponse)
	if !bytes.Equal(imageResponse.Body.Bytes(), pngData.Bytes()) { t.Fatal("project image contents differ") }
	want(jsonRequest("GET", base+"/showcase/"+projectID+"/images/not-owned.png", "", ""), http.StatusNotFound)
	publicProjects := jsonRequest("GET", base+"/showcase", "", "")
	want(publicProjects, http.StatusOK)
	noCache(publicProjects)
	if !strings.Contains(publicProjects.Body.String(), projectID) { t.Fatalf("published project missing: %s", publicProjects.Body.String()) }
	want(multipartRequest("PATCH", base+"/manage/showcase/"+projectID, adminToken, map[string][]string{"title": {"Project"}, "published": {"false"}, "keepImages": projectImages}, 0), http.StatusOK)
	want(jsonRequest("GET", base+"/showcase/"+projectID, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", projectImage, "", ""), http.StatusNotFound)
	currentETag := jsonRequest("GET", avatar, "", "").Header().Get("ETag")
	want(jsonRequest("PATCH", base+"/me/profile", memberToken, `{"profilePublic":false}`), http.StatusOK)
	want(conditional(avatar, "", currentETag), http.StatusNotFound)
	want(jsonRequest("GET", base+"/members/"+member.Id, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", cardImage, "", ""), http.StatusNotFound)
	want(jsonRequest("GET", avatar, "", ""), http.StatusNotFound)
	want(jsonRequest("DELETE", base+"/me/cards/"+cardID, memberToken, ""), http.StatusNoContent)
	want(jsonRequest("DELETE", base+"/me/cards/"+cardID, memberToken, ""), http.StatusNotFound)
	want(jsonRequest("DELETE", base+"/manage/showcase/"+projectID, adminToken, ""), http.StatusNoContent)
	want(jsonRequest("GET", projectImage, "", ""), http.StatusNotFound)
}
