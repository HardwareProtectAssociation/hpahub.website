package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

type invitation struct {
	Code      string `json:"code" db:"code"`
	Created   int64  `json:"created" db:"created"`
	Expires   int64  `json:"expires" db:"expires"`
	Used      int64  `json:"used" db:"used"`
	UsedBy    string `json:"usedBy" db:"used_by"`
	CreatedBy string `json:"createdBy" db:"created_by"`
	MaxUses   int    `json:"maxUses" db:"max_uses"`
	UseCount  int    `json:"useCount" db:"use_count"`
}

var errInvalidInvitation = errors.New("invalid or expired invitation")

func initInvitations(app core.App) error {
	_, err := app.DB().NewQuery(`CREATE TABLE IF NOT EXISTS hpahub_invitations (
		code TEXT PRIMARY KEY, created INTEGER NOT NULL, expires INTEGER NOT NULL,
		used INTEGER NOT NULL DEFAULT 0, used_by TEXT NOT NULL DEFAULT '', created_by TEXT NOT NULL DEFAULT '',
		max_uses INTEGER NOT NULL DEFAULT 1, use_count INTEGER NOT NULL DEFAULT 0
	)`).Execute()
	if err != nil {
		return err
	}
	// Add new columns to existing tables
	var maxUsesCol int
	if err := app.DB().NewQuery("SELECT COUNT(*) FROM pragma_table_info('hpahub_invitations') WHERE name = 'max_uses'").Row(&maxUsesCol); err == nil && maxUsesCol == 0 {
		if _, err := app.DB().NewQuery("ALTER TABLE hpahub_invitations ADD COLUMN max_uses INTEGER NOT NULL DEFAULT 1").Execute(); err != nil {
			return err
		}
	}
	var useCountCol int
	if err := app.DB().NewQuery("SELECT COUNT(*) FROM pragma_table_info('hpahub_invitations') WHERE name = 'use_count'").Row(&useCountCol); err == nil && useCountCol == 0 {
		if _, err := app.DB().NewQuery("ALTER TABLE hpahub_invitations ADD COLUMN use_count INTEGER NOT NULL DEFAULT 0").Execute(); err != nil {
			return err
		}
	}
	// Initialize use_count for existing used invitations
	if _, err := app.DB().NewQuery("UPDATE hpahub_invitations SET use_count = 1 WHERE used > 0 AND use_count = 0").Execute(); err != nil {
		return err
	}
	var creatorColumn int
	if err := app.DB().NewQuery("SELECT COUNT(*) FROM pragma_table_info('hpahub_invitations') WHERE name = 'created_by'").Row(&creatorColumn); err != nil {
		return err
	}
	if creatorColumn == 0 {
		if _, err := app.DB().NewQuery("ALTER TABLE hpahub_invitations ADD COLUMN created_by TEXT NOT NULL DEFAULT ''").Execute(); err != nil {
			return err
		}
	}
	_, err = app.DB().NewQuery(`CREATE TABLE IF NOT EXISTS hpahub_auth_limits (
		kind TEXT PRIMARY KEY, window INTEGER NOT NULL, count INTEGER NOT NULL
	)`).Execute()
	return err
}

// ponytail: global limits cap total traffic; use a shared per-client and global limiter when traffic grows.
func allowAuthAttempt(app core.App, kind string, limit int) (bool, error) {
	allowed := false
	err := app.RunInTransaction(func(tx core.App) error {
		window := time.Now().Unix() / 60
		_, err := tx.DB().NewQuery(`INSERT INTO hpahub_auth_limits (kind, window, count) VALUES ({:kind}, {:window}, 1)
			ON CONFLICT(kind) DO UPDATE SET count = CASE WHEN window = excluded.window THEN count + 1 ELSE 1 END, window = excluded.window`).Bind(dbx.Params{"kind": kind, "window": window}).Execute()
		if err != nil {
			return err
		}
		var count int
		if err := tx.DB().NewQuery("SELECT count FROM hpahub_auth_limits WHERE kind = {:kind}").Bind(dbx.Params{"kind": kind}).Row(&count); err != nil {
			return err
		}
		allowed = count <= limit
		return nil
	})
	return allowed, err
}

func newInvitation(app core.App, now time.Time, createdBy string) (invitation, error) {
	return newInvitationWithUses(app, now, createdBy, 1, 24*time.Hour)
}

func newInvitationWithUses(app core.App, now time.Time, createdBy string, maxUses int, ttl time.Duration) (invitation, error) {
	// 32 characters allow uniform sampling without modulo bias.
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	expires := int64(0)
	if ttl > 0 {
		expires = now.Add(ttl).Unix()
	}
	for range 10 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return invitation{}, err
		}
		code := []byte("HPA-00000000")
		for j, b := range random {
			code[j+4] = alphabet[int(b)&31]
		}
		item := invitation{Code: string(code), Created: now.Unix(), Expires: expires, CreatedBy: createdBy, MaxUses: maxUses}
		result, err := app.DB().NewQuery("INSERT OR IGNORE INTO hpahub_invitations (code, created, expires, created_by, max_uses) VALUES ({:code}, {:created}, {:expires}, {:createdBy}, {:maxUses})").Bind(dbx.Params{"code": item.Code, "created": item.Created, "expires": item.Expires, "createdBy": item.CreatedBy, "maxUses": maxUses}).Execute()
		if err != nil {
			return invitation{}, err
		}
		if n, err := result.RowsAffected(); err != nil {
			return invitation{}, err
		} else if n == 1 {
			return item, nil
		}
	}
	return invitation{}, errors.New("could not generate a unique invitation")
}

func registerInvitation(app core.App, email, password, confirm, code string, now time.Time, event *core.RequestEvent) (*core.Record, error) {
	if password != confirm || password == "" || email == "" {
		return nil, errors.New("invalid registration")
	}
	var user *core.Record
	err := app.RunInTransaction(func(tx core.App) error {
		if _, err := tx.FindAuthRecordByEmail(core.CollectionNameSuperusers, email); err == nil {
			return errors.New("invalid registration")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		result, err := tx.DB().NewQuery(`UPDATE hpahub_invitations SET used = CASE WHEN max_uses = 1 THEN {:now} ELSE used END,
			used_by = {:email}, use_count = use_count + 1
			WHERE code = {:code} AND max_uses > 0 AND use_count < max_uses AND expires > {:now}`).Bind(dbx.Params{"now": now.Unix(), "email": email, "code": code}).Execute()
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return errInvalidInvitation
		}
		collection, err := tx.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		if !collection.PasswordAuth.Enabled {
			return errors.New("password authentication disabled")
		}
		user = core.NewRecord(collection)
		user.SetEmail(email)
		user.Set("role", "member")
		user.SetPassword(password)
		if err := tx.Save(user); err != nil {
			return err
		}
		if event != nil {
			originalApp, originalResponse := event.App, event.Response
			buffer := &router.ResponseWriter{ResponseWriter: httptest.NewRecorder()}
			event.App, event.Response = tx, buffer
			defer func() { event.App, event.Response = originalApp, originalResponse }()
			if err := apis.RecordAuthResponse(event, user, core.MFAMethodPassword, nil); err != nil {
				return err
			}
			if buffer.Written() {
				return errors.New("authentication did not produce a token")
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func siteRole(record *core.Record) string {
	if record == nil {
		return "guest"
	}
	switch record.Collection().Name {
	case core.CollectionNameSuperusers:
		return "octopus"
	case "users":
		switch record.GetString("role") {
		case "admin", "member":
			return record.GetString("role")
		}
	}
	return "user"
}

// devShortcutAllowed gates the 1/1 login shortcut to explicit local
// development runs only: the feature flag must be set and the request must
// come from the loopback interface. Production never sets HPA_DEV_LOGIN.
func devShortcutAllowed(e *core.RequestEvent) bool {
	if os.Getenv("HPA_DEV_LOGIN") != "1" {
		return false
	}
	host, _, err := net.SplitHostPort(e.Request.RemoteAddr)
	if err != nil {
		host = e.Request.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func devShortcutRecord(app core.App) (*core.Record, error) {
	record, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "admin@hpahub.local")
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	records, err := app.FindRecordsByFilter(core.CollectionNameSuperusers, "", "created", 1, 0)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

func bindInvitationRoutes(e *core.ServeEvent) {
	limited := func(e *core.RequestEvent, kind string, limit int) error {
		ok, err := allowAuthAttempt(e.App, kind, limit)
		if err != nil {
			return e.InternalServerError("Unable to check request rate.", err)
		}
		if !ok {
			return e.TooManyRequestsError("Too many requests. Try again later.", nil)
		}
		return nil
	}
	e.Router.GET("/api/hpahub/me", func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		return e.JSON(http.StatusOK, map[string]string{"id": e.Auth.Id, "role": siteRole(e.Auth)})
	})
	e.Router.POST("/api/hpahub/login", func(e *core.RequestEvent) error {
		if err := limited(e, "login", 60); err != nil {
			return err
		}
		var input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid credentials.", nil)
		}
		if strings.TrimSpace(input.Email) == "1" && input.Password == "1" && devShortcutAllowed(e) {
			if record, err := devShortcutRecord(e.App); err == nil && record != nil {
				e.App.Logger().Warn("development 1/1 login shortcut used", "email", record.Email())
				return apis.RecordAuthResponse(e, record, core.MFAMethodPassword, nil)
			}
		}
		email := strings.TrimSpace(input.Email)
		if email == "admin" && os.Getenv("HPA_LOCAL_TEST") == "1" {
			email = "admin@hpahub.local"
		}
		record, err := e.App.FindAuthRecordByEmail(core.CollectionNameSuperusers, email)
		if errors.Is(err, sql.ErrNoRows) {
			record, err = e.App.FindAuthRecordByEmail("users", email)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return e.InternalServerError("Authentication failed.", err)
		}
		if errors.Is(err, sql.ErrNoRows) || record == nil || !record.ValidatePassword(input.Password) {
			return e.BadRequestError("Invalid credentials.", nil)
		}
		if !record.Collection().PasswordAuth.Enabled {
			return e.ForbiddenError("Password authentication disabled.", nil)
		}
		return apis.RecordAuthResponse(e, record, core.MFAMethodPassword, nil)
	})
	e.Router.POST("/api/hpahub/register", func(e *core.RequestEvent) error {
		if err := limited(e, "register", 30); err != nil {
			return err
		}
		var input struct {
			Email           string `json:"email"`
			Password        string `json:"password"`
			PasswordConfirm string `json:"passwordConfirm"`
			InviteCode      string `json:"inviteCode"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid registration.", nil)
		}
		_, err := registerInvitation(e.App, strings.TrimSpace(input.Email), input.Password, input.PasswordConfirm, input.InviteCode, time.Now(), e)
		if err != nil {
			return e.BadRequestError("Invalid registration or invitation.", nil)
		}
		return nil
	})
	e.Router.POST("/api/hpahub/invitations", func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "member" && role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		var input struct {
			Count   int  `json:"count"`
			MaxUses *int `json:"maxUses"`
		}
		if e.Request.ContentLength > 0 {
			if err := e.BindBody(&input); err != nil {
				return e.BadRequestError("Invalid invitation count.", nil)
			}
		}
		if input.Count == 0 {
			input.Count = 1
		}
		if input.Count < 1 || input.Count > 5 {
			return e.BadRequestError("You can generate 1 to 5 invitations at a time.", nil)
		}
		maxUses := 1
		if input.MaxUses != nil {
			maxUses = *input.MaxUses
		}
		if maxUses < 1 || maxUses > 1000 {
			return e.BadRequestError("Invalid max uses.", nil)
		}
		ttl := 24 * time.Hour
		items := make([]invitation, 0, input.Count)
		for range input.Count {
			item, err := newInvitationWithUses(e.App, time.Now(), e.Auth.Id, maxUses, ttl)
			if err != nil {
				return e.InternalServerError("Failed to create invitation.", err)
			}
			items = append(items, item)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	})
	e.Router.GET("/api/hpahub/invitations", func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "member" && role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		items := []invitation{}
		sqlText := "SELECT code, created, expires, used, used_by, created_by, max_uses, use_count FROM hpahub_invitations"
		params := dbx.Params{}
		if role == "member" {
			sqlText += " WHERE created_by = {:user}"
			params["user"] = e.Auth.Id
		}
		sqlText += " ORDER BY created DESC, code DESC"
		query := e.App.DB().NewQuery(sqlText)
		if len(params) > 0 {
			query = query.Bind(params)
		}
		if err := query.All(&items); err != nil {
			return e.InternalServerError("Failed to load invitations.", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	})
}

func bindNativeAuthLimit(app core.App) {
	app.OnRecordAuthWithPasswordRequest().BindFunc(func(e *core.RecordAuthWithPasswordRequestEvent) error {
		ok, err := allowAuthAttempt(e.App, "login", 60)
		if err != nil {
			return e.InternalServerError("Unable to check request rate.", err)
		}
		if !ok {
			return e.TooManyRequestsError("Too many requests. Try again later.", nil)
		}
		return e.Next()
	})
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.IsNewRecord {
			return e.ForbiddenError("Invitation required.", nil)
		}
		return e.Next()
	})
}
