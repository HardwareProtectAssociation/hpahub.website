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

func TestHardwareNewsReactions(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	author := centerTestUser(t, app, "news-author@example.com", "member")
	reader := centerTestUser(t, app, "news-reader@example.com", "member")
	authorToken, _ := author.NewAuthToken()
	readerToken, _ := reader.NewAuthToken()
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindNewsRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	response := call("POST", "/api/hpahub/news", authorToken, `{"title":"新键盘固件","content":"今天发布了一个开源固件。","link":"https://example.com/news"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("publish status = %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("publish response: %s", response.Body.String())
	}
	response = call("GET", "/api/hpahub/news", readerToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", response.Code, response.Body.String())
	}
	var list struct {
		Items []hardwareNewsItem `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("items = %+v, %v", list.Items, err)
	}
	if list.Items[0].IsAuthor || list.Items[0].CanDelete {
		t.Fatalf("unexpected reader permissions: %+v", list.Items[0])
	}
	path := "/api/hpahub/news/" + created.ID + "/reaction"
	if response := call("POST", path, authorToken, `{"reaction":"like"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("self reaction status = %d, want 400", response.Code)
	}
	response = call("POST", path, readerToken, `{"reaction":"like"}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"likeCount":1`) {
		t.Fatalf("like = %d: %s", response.Code, response.Body.String())
	}
	response = call("GET", "/api/hpahub/news", readerToken, "")
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].LikeCount != 1 || list.Items[0].MyReaction != "like" {
		t.Fatalf("list after reaction = %s, %v", response.Body.String(), err)
	}
	response = call("POST", path, readerToken, `{"reaction":"useless"}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"uselessCount":1`) || !strings.Contains(response.Body.String(), `"likeCount":0`) {
		t.Fatalf("switch reaction = %d: %s", response.Code, response.Body.String())
	}
	if response := call("DELETE", "/api/hpahub/news/"+created.ID, readerToken, ""); response.Code != http.StatusForbidden {
		t.Fatalf("reader delete status = %d, want 403", response.Code)
	}
	if response := call("DELETE", "/api/hpahub/news/"+created.ID, authorToken, ""); response.Code != http.StatusOK {
		t.Fatalf("author delete status = %d: %s", response.Code, response.Body.String())
	}
}
