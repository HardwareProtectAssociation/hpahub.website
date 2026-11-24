package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
)

const mediaCookieName = "hpa_warehouse_media"
const mediaCookiePath = "/api/hpahub/warehouse/media/"
const mediaPurpose = "hpahub-warehouse-media"
const legacyBodyLimit int64 = 500 << 20

func internalRole(record *core.Record) bool {
	r := siteRole(record)
	return r == "member" || r == "admin" || r == "octopus"
}

func mediaKey(record *core.Record) string {
	return record.TokenKey() + record.Collection().AuthToken.Secret + ":" + mediaPurpose
}
func mediaCookie(e *core.RequestEvent, value string, expires time.Time, maxAge int) {
	secure := os.Getenv("HPA_LOCAL_TEST") != "1" || e.Request.TLS != nil
	http.SetCookie(e.Response, &http.Cookie{Name: mediaCookieName, Value: value, Path: mediaCookiePath, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge})
}

func mediaSession(e *core.RequestEvent) error {
	if e.Request.Method == http.MethodDelete {
		mediaCookie(e, "", time.Unix(1, 0), -1)
		return e.NoContent(http.StatusNoContent)
	}
	if e.Auth == nil { return e.UnauthorizedError("Authentication required.", nil) }
	if !internalRole(e.Auth) { return e.ForbiddenError("Insufficient permissions.", nil) }
	authorization := e.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") { return e.UnauthorizedError("Authentication required.", nil) }
	token := strings.TrimPrefix(authorization, "Bearer ")
	claims, err := security.ParseUnverifiedJWT(token)
	if err != nil { return e.UnauthorizedError("Authentication required.", nil) }
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil { return e.UnauthorizedError("Authentication required.", nil) }
	deadline := time.Now().Add(15 * time.Minute)
	if exp.Time.Before(deadline) { deadline = exp.Time }
	if !deadline.After(time.Now()) { return e.UnauthorizedError("Authentication required.", nil) }
	payload := jwt.MapClaims{"collection": e.Auth.Collection().Name, "id": e.Auth.Id, "purpose": mediaPurpose, "exp": deadline.Unix()}
	value, err := security.NewJWT(payload, mediaKey(e.Auth), time.Until(deadline))
	if err != nil { return e.InternalServerError("Unable to create media session.", err) }
	mediaCookie(e, value, deadline, int(time.Until(deadline).Seconds()))
	return e.NoContent(http.StatusNoContent)
}

func mediaIdentity(e *core.RequestEvent) *core.Record {
	cookie, err := e.Request.Cookie(mediaCookieName)
	if err != nil { return nil }
	claims, err := security.ParseUnverifiedJWT(cookie.Value)
	if err != nil { return nil }
	collection, _ := claims["collection"].(string)
	id, _ := claims["id"].(string)
	if collection != "users" && collection != core.CollectionNameSuperusers { return nil }
	record, err := e.App.FindRecordById(collection, id)
	if err != nil || !internalRole(record) { return nil }
	claims, err = security.ParseJWT(cookie.Value, mediaKey(record))
	if err != nil || claims["purpose"] != mediaPurpose { return nil }
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil || !exp.After(time.Now()) { return nil }
	return record
}

func validSegment(s string) bool {
	if s == "" || s == "." || s == ".." || len(s) > 1024 || strings.ContainsAny(s, "/\\") { return false }
	for _, r := range s { if unicode.IsControl(r) { return false } }
	return true
}

func legacySegments(r *http.Request, prefix string) ([]string, bool) {
	path := r.URL.EscapedPath()
	if !strings.HasPrefix(path, prefix) { return nil, false }
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	for i, part := range parts {
		value, err := url.PathUnescape(part)
		if err != nil || !validSegment(value) { return nil, false }
		parts[i] = value
	}
	return parts, true
}

// legacyRoute returns access level: 0 public, 1 internal, 2 octopus, -1 denied.
func legacyRoute(service, method string, p []string) int {
	if service == "gpu" {
		if len(p) == 1 && p[0] == "related-media" && (method == "GET" || method == "POST") { return 1 }
		if len(p) == 1 && p[0] == "stats" && method == "GET" { return 0 }
		if len(p) == 2 && p[0] == "options" && p[1] == "games" { if method == "GET" { return 0 }; if method == "POST" { return 1 } }
		if len(p) == 1 && p[0] == "reorder" && method == "POST" { return 1 }
		if len(p) >= 1 && (p[0] == "gpus" || p[0] == "items") {
			if len(p) == 1 { if method == "GET" { return 0 }; if method == "POST" { return 1 } }
			if len(p) >= 2 { if id, err := strconv.ParseUint(p[1], 10, 64); err != nil || id == 0 { return -1 } }
			if len(p) == 2 { if method == "GET" { return 0 }; if method == "PUT" || method == "DELETE" { return 1 } }
			if len(p) == 3 && p[2] == "copy" && method == "POST" { return 1 }
		}
		return -1
	}
	if len(p) == 1 && p[0] == "storage" { if method == "GET" { return 1 }; if method == "PUT" || method == "PATCH" { return 2 } }
	if len(p) == 1 && p[0] == "projects" && (method == "GET" || method == "POST") { return 1 }
	if len(p) >= 2 && p[0] == "projects" && !strings.HasPrefix(p[1], ".") {
		if len(p) == 2 && (method == "GET" || method == "PUT" || method == "PATCH" || method == "DELETE") { return 1 }
		if len(p) == 3 && p[2] == "files" && method == "POST" { return 1 }
		if len(p) >= 5 && p[2] == "files" && validCategory(p[3]) && !strings.HasPrefix(p[4], ".") {
			if len(p) == 5 && (method == "PUT" || method == "PATCH" || method == "DELETE") { return 1 }
			if len(p) == 6 && p[5] == "transcode" && method == "POST" { return 1 }
		}
	}
	return -1
}

func validCategory(s string) bool { return s == "images" || s == "videos" || s == "docs" || s == "others" }

func stripRelatedMedia(value any) {
	switch v := value.(type) {
	case map[string]any:
		delete(v, "related_media")
		for _, child := range v { stripRelatedMedia(child) }
	case []any:
		for _, child := range v { stripRelatedMedia(child) }
	}
}

func proxyLegacy(e *core.RequestEvent, service string) error {
	prefix := "/api/hpahub/" + service + "/"
	parts, ok := legacySegments(e.Request, prefix)
	if !ok { return e.NotFoundError("Not found.", nil) }
	media := service == "warehouse" && len(parts) == 5 && parts[0] == "media"
	access := -1
	if media {
		if (e.Request.Method == "GET" || e.Request.Method == "HEAD") && (parts[1] == "file" || parts[1] == "thumb" || parts[1] == "web" || parts[1] == "preview") && validCategory(parts[3]) && !strings.HasPrefix(parts[2], ".") && !strings.HasPrefix(parts[4], ".") { access = 1 }
	} else { access = legacyRoute(service, e.Request.Method, parts) }
	if access < 0 { return e.NotFoundError("Not found.", nil) }
	identity := e.Auth
	if media { identity = mediaIdentity(e) }
	if access > 0 && identity == nil { return e.UnauthorizedError("Authentication required.", nil) }
	if access > 0 && !internalRole(identity) { return e.ForbiddenError("Insufficient permissions.", nil) }
	if access == 2 && siteRole(identity) != "octopus" { return e.ForbiddenError("Insufficient permissions.", nil) }
	base := "http://gpu:13001"
	upstream := append([]string{"api"}, parts...)
	if service == "warehouse" { base = "http://warehouse:18080" }
	if media { upstream = append([]string{"api", "hpahub"}, parts...) }
	for i := range upstream { upstream[i] = url.PathEscape(upstream[i]) }
	target := base + "/" + strings.Join(upstream, "/")
	if e.Request.URL.RawQuery != "" && !media { target += "?" + e.Request.URL.RawQuery }
	req, err := http.NewRequestWithContext(e.Request.Context(), e.Request.Method, target, e.Request.Body)
	if err != nil { return e.BadRequestError("Invalid request.", err) }
	req.ContentLength = e.Request.ContentLength
	for _, key := range []string{"Content-Type", "Accept", "Range", "If-Range"} { if v := e.Request.Header.Get(key); v != "" { req.Header.Set(key, v) } }
	if service == "warehouse" && identity != nil {
		req.Header.Set("X-User-Id", identity.Id)
		name := identity.GetString("name")
		if name == "" { name = identity.Email() }
		if name == "" { name = identity.Id }
		req.Header.Set("X-User-Name", url.PathEscape(name))
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, apis.ErrRequestEntityTooLarge) { return e.Error(http.StatusRequestEntityTooLarge, "Request too large.", err) }
		return e.Error(http.StatusBadGateway, "Upstream unavailable.", err)
	}
	defer resp.Body.Close()
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "ETag", "Last-Modified"} { if v := resp.Header.Get(key); v != "" { e.Response.Header().Set(key, v) } }
	if media {
		e.Response.Header().Set("Cache-Control", "private, no-store")
		e.Response.Header().Set("X-Content-Type-Options", "nosniff")
		contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		switch contentType { case "image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/webm", "video/ogg":
			e.Response.Header().Set("Content-Disposition", "inline")
		default:
			e.Response.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": parts[4]}))
		}
	}
	if service == "gpu" && !internalRole(identity) && e.Request.Method == "GET" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data any
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil { return e.Error(http.StatusBadGateway, "Invalid upstream response.", err) }
		stripRelatedMedia(data)
		encoded, err := json.Marshal(data)
		if err != nil { return e.Error(http.StatusBadGateway, "Invalid upstream response.", err) }
		e.Response.Header().Set("Content-Length", fmt.Sprint(len(encoded)))
		e.Response.WriteHeader(resp.StatusCode)
		_, err = e.Response.Write(encoded)
		return err
	}
	e.Response.WriteHeader(resp.StatusCode)
	_, err = io.Copy(e.Response, resp.Body)
	return err
}

func bindLegacyProxyRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/warehouse/media-session", mediaSession)
	e.Router.DELETE("/api/hpahub/warehouse/media-session", mediaSession)
	for _, service := range []string{"gpu", "warehouse"} {
		service := service
		handler := func(e *core.RequestEvent) error { return proxyLegacy(e, service) }
		path := "/api/hpahub/" + service + "/{path...}"
		for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"} {
			e.Router.Route(method, path, handler).Bind(&hook.Handler[*core.RequestEvent]{
				Priority: apis.DefaultBodyLimitMiddlewarePriority - 1,
				Func: func(e *core.RequestEvent) error {
					// PocketBase's rereadable wrapper buffers and rewinds the body.
					// Sidecar uploads must instead remain a one-pass stream.
					if body, ok := e.Request.Body.(*router.RereadableReadCloser); ok {
						e.Request.Body = body.ReadCloser
					}
					return e.Next()
				},
			}, apis.BodyLimit(legacyBodyLimit))
		}
	}
}
