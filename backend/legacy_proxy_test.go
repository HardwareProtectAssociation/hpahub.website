package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type proxyTransport func(*http.Request) (*http.Response, error)
func (f proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLegacyProxySecurity(t *testing.T) {
	t.Setenv("HPA_LOCAL_TEST", "")
	app := invitationTestApp(t)
	router, err := apis.NewRouter(app)
	if err != nil { t.Fatal(err) }
	bindLegacyProxyRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil { t.Fatal(err) }
	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil { t.Fatal(err) }
	record := core.NewRecord(collection)
	record.SetEmail("proxy-test@example.com")
	record.SetPassword("valid-password-123")
	record.Set("name", "测试 名称")
	record.Set("role", "member")
	if err := app.Save(record); err != nil { t.Fatal(err) }
	token, err := record.NewAuthToken()
	if err != nil { t.Fatal(err) }
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	var upstream *http.Request
	var uploadBody string
	http.DefaultTransport = proxyTransport(func(r *http.Request) (*http.Response, error) {
		upstream = r
		if r.Method == "POST" && r.URL.Host == "warehouse:18080" {
			data, err := io.ReadAll(io.LimitReader(r.Body, r.ContentLength+1))
			if err != nil { t.Fatal(err) }
			uploadBody = string(data)
		}
		header := http.Header{"Content-Type": {"application/json"}, "Set-Cookie": {"upstream=secret"}}
		body := `[{"related_media":["private"],"nested":{"related_media":"secret"},"name":"gpu"}]`
		if r.URL.Host == "warehouse:18080" { header.Set("Content-Type", "text/html"); body = "<script>bad</script>" }
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	request := func(method, path, auth string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if auth != "" { r.Header.Set("Authorization", "Bearer " + auth) }
		r.Header.Set("X-User-Id", "spoof")
		r.Header.Set("Cookie", "spoof=secret")
		if cookie != nil { r.AddCookie(cookie) }
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/api/hpahub/gpu/gpus", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "related_media") || strings.Contains(w.Body.String(), "private") { t.Fatalf("public GPU: %d %s", w.Code, w.Body.String()) }
	if upstream.URL.String() != "http://gpu:13001/api/gpus" || upstream.Header.Get("Cookie") != "" || upstream.Header.Get("Authorization") != "" || upstream.Header.Get("X-User-Id") != "" || w.Header().Get("Set-Cookie") != "" { t.Fatal("GPU proxy leaked headers or used wrong target") }
	if w := request("POST", "/api/hpahub/gpu/gpus", "", nil); w.Code != 401 { t.Fatalf("anonymous GPU write: %d", w.Code) }
	if w := request("GET", "/api/hpahub/gpu/related-media", "", nil); w.Code != 401 { t.Fatalf("anonymous related media: %d", w.Code) }
	if w := request("GET", "/api/hpahub/gpu/gpus", token, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "related_media") { t.Fatalf("member GPU data: %d %s", w.Code, w.Body.String()) }
	if w := request("GET", "/api/hpahub/gpu/gpus/1/extra", token, nil); w.Code != 404 { t.Fatalf("unknown route: %d", w.Code) }
	if w := request("PUT", "/api/hpahub/warehouse/storage", token, nil); w.Code != 403 { t.Fatalf("member storage write: %d", w.Code) }
	multipartBody := "--boundary\r\nContent-Disposition: form-data; name=\"files\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nhello\r\n--boundary--\r\n"
	upload := httptest.NewRequest("POST", "/api/hpahub/warehouse/projects/测试项目/files", strings.NewReader(multipartBody))
	upload.Header.Set("Authorization", "Bearer " + token)
	upload.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	uploadResponse := httptest.NewRecorder()
	mux.ServeHTTP(uploadResponse, upload)
	if uploadResponse.Code != 200 || uploadBody != multipartBody || upstream.Header.Get("Content-Type") != upload.Header.Get("Content-Type") { t.Fatalf("multipart upload: status=%d body=%q", uploadResponse.Code, uploadBody) }
	oversized := httptest.NewRequest("POST", "/api/hpahub/warehouse/projects/project/files", nil)
	oversized.Header.Set("Authorization", "Bearer " + token)
	oversized.ContentLength = legacyBodyLimit + 1
	tooLarge := httptest.NewRecorder()
	mux.ServeHTTP(tooLarge, oversized)
	if tooLarge.Code != http.StatusRequestEntityTooLarge { t.Fatalf("oversized upload: %d", tooLarge.Code) }
	w = request("POST", "/api/hpahub/warehouse/media-session", token, nil)
	if w.Code != 204 || len(w.Result().Cookies()) != 1 { t.Fatalf("media session: %d %s", w.Code, w.Body.String()) }
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != mediaCookiePath || cookie.MaxAge <= 0 || cookie.MaxAge > 900 { t.Fatalf("cookie attributes: %+v", cookie) }
	t.Setenv("HPA_LOCAL_TEST", "1")
	localSession := request("POST", "/api/hpahub/warehouse/media-session", token, nil)
	if localSession.Code != 204 || len(localSession.Result().Cookies()) != 1 || localSession.Result().Cookies()[0].Secure { t.Fatal("local HTTP cookie must not require TLS") }
	t.Setenv("HPA_LOCAL_TEST", "")
	path := "/api/hpahub/warehouse/media/file/项目/images/a.html"
	if w := request("GET", path, token, nil); w.Code != 401 { t.Fatalf("bearer used as media cookie: %d", w.Code) }
	w = request("GET", path, "", cookie)
	if w.Code != 200 || upstream.URL.EscapedPath() != "/api/hpahub/media/file/%E9%A1%B9%E7%9B%AE/images/a.html" || upstream.Header.Get("X-User-Id") != record.Id || upstream.Header.Get("Cookie") != "" { t.Fatalf("media request: %d %#v", w.Code, upstream) }
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" { t.Fatal("unsafe media response") }
	if upstream.Header.Get("X-User-Name") != "%E6%B5%8B%E8%AF%95%20%E5%90%8D%E7%A7%B0" { t.Fatalf("encoded user name: %q", upstream.Header.Get("X-User-Name")) }
	if w := request("GET", "/api/hpahub/warehouse/media/file/项目/images/a%2Fb", "", cookie); w.Code != 404 { t.Fatalf("encoded separator accepted: %d", w.Code) }
	badCookie := *cookie
	badCookie.Value += "x"
	if w := request("GET", path, "", &badCookie); w.Code != 401 { t.Fatalf("tampered cookie: %d", w.Code) }
	if w := request("GET", "/api/hpahub/warehouse/media/file/项目/images/.meta.json", "", cookie); w.Code != 404 { t.Fatalf("hidden media: %d", w.Code) }
	record.Set("role", "user")
	if err := app.Save(record); err != nil { t.Fatal(err) }
	if w := request("GET", path, "", cookie); w.Code != 401 { t.Fatalf("demoted user media: %d", w.Code) }
	w = request("DELETE", "/api/hpahub/warehouse/media-session", "", nil)
	if w.Code != 204 || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge >= 0 { t.Fatal("media cookie not deleted") }
}

func TestLegacyRouteWhitelist(t *testing.T) {
	for _, tc := range []struct{ service, method string; path []string; want int }{
		{"gpu", "GET", []string{"gpus", "1"}, 0},
		{"gpu", "GET", []string{"gpus", "1", "copy"}, -1},
		{"gpu", "POST", []string{"items", "1", "copy"}, 1},
		{"warehouse", "POST", []string{"projects", "project", "files"}, 1},
		{"warehouse", "GET", []string{"projects", "project", "files", "images", "a"}, -1},
		{"warehouse", "POST", []string{"projects", "project", "files", "videos", "a", "transcode"}, 1},
		{"warehouse", "PATCH", []string{"storage"}, 2},
	} { if got := legacyRoute(tc.service, tc.method, tc.path); got != tc.want { t.Fatalf("%s %s %v: got %d want %d", tc.service, tc.method, tc.path, got, tc.want) } }
	for _, s := range []string{"", ".", "..", "a/b", "a\\b", "a\x00b"} { if validSegment(s) { t.Fatalf("accepted segment %q", s) } }
}
