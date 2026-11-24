package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/spf13/cobra"
	_ "golang.org/x/image/webp"
)

type legacyAccount struct {
	id int64
	username, hash string
	admin int64
	avatar []byte
	mime sql.NullString
}

func validLegacyHash(hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "scrypt:32768:8:1" || parts[1] == "" || !utf8.ValidString(parts[1]) || len(parts[2]) != 128 { return false }
	_, err := hex.DecodeString(parts[2])
	return err == nil
}

func readLegacyAccounts(source string) ([]legacyAccount, error) {
	path, err := filepath.Abs(source)
	if err != nil { return nil, err }
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil { return nil, err }
	defer db.Close()
	tx, err := db.Begin()
	if err != nil { return nil, err }
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id, username, password_hash, is_admin, avatar, avatar_mime,
		typeof(id), typeof(username), typeof(password_hash), typeof(is_admin), typeof(avatar), typeof(avatar_mime) FROM users ORDER BY id`)
	if err != nil { return nil, err }
	defer rows.Close()
	var accounts []legacyAccount
	ids := map[int64]bool{}
	names := map[string]bool{}
	for rows.Next() {
		var a legacyAccount
		var idType, nameType, hashType, adminType, avatarType, mimeType string
		if err := rows.Scan(&a.id, &a.username, &a.hash, &a.admin, &a.avatar, &a.mime, &idType, &nameType, &hashType, &adminType, &avatarType, &mimeType); err != nil { return nil, err }
		if idType != "integer" || a.id <= 0 || ids[a.id] || nameType != "text" || hashType != "text" || adminType != "integer" || (avatarType != "blob" && avatarType != "null") || (mimeType != "text" && mimeType != "null") {
			return nil, fmt.Errorf("invalid legacy account data at ID %d", a.id)
		}
		if !utf8.ValidString(a.username) || a.username == "" || strings.TrimSpace(a.username) != a.username || strings.ContainsAny(a.username, "@\x00") || strings.IndexFunc(a.username, unicode.IsControl) >= 0 || utf8.RuneCountInString(a.username) > 255 || names[a.username] {
			return nil, fmt.Errorf("invalid or duplicate legacy username at ID %d", a.id)
		}
		if !validLegacyHash(a.hash) { return nil, fmt.Errorf("unsupported legacy password hash at ID %d", a.id) }
		ids[a.id], names[a.username] = true, true
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil { return nil, err }
	if err := rows.Close(); err != nil { return nil, err }
	if err := tx.Commit(); err != nil { return nil, err }
	return accounts, nil
}

func legacyAvatar(a legacyAccount) (*filesystem.File, error) {
	if len(a.avatar) == 0 || len(a.avatar) > 5*1024*1024 || !a.mime.Valid { return nil, nil }
	mime := http.DetectContentType(a.avatar)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}[mime]
	if ext == "" || a.mime.String != mime { return nil, nil }
	if _, _, err := image.DecodeConfig(bytes.NewReader(a.avatar)); err != nil { return nil, nil }
	return filesystem.NewFileFromBytes(a.avatar, "avatar"+ext)
}

func importLegacyAccounts(app core.App, source string) error {
	accounts, err := readLegacyAccounts(source)
	if err != nil { return err }
	var uploaded []string
	err = app.RunInTransaction(func(tx core.App) error {
		collection, err := tx.FindCollectionByNameOrId("users")
		if err != nil { return err }
		// Check every existing mapping before creating records or uploading files.
		for _, a := range accounts {
			var existing struct { UserID string `db:"user_id"`; Username string `db:"username"` }
			err := tx.DB().NewQuery("SELECT user_id, username FROM hpahub_legacy_accounts WHERE legacy_id = {:id}").Bind(dbx.Params{"id": a.id}).One(&existing)
			if err != nil && !errors.Is(err, sql.ErrNoRows) { return err }
			if err == nil {
				if existing.Username != a.username { return fmt.Errorf("legacy account ownership mismatch at ID %d", a.id) }
				if _, err := tx.FindRecordById(collection, existing.UserID); err != nil { return fmt.Errorf("legacy account ownership mismatch at ID %d: %w", a.id, err) }
				continue
			}
			var count int
			if err := tx.DB().NewQuery("SELECT count(*) FROM hpahub_legacy_accounts WHERE username = {:username}").Bind(dbx.Params{"username": a.username}).Row(&count); err != nil { return err }
			if count != 0 { return fmt.Errorf("legacy username conflict at ID %d", a.id) }
		}
		for _, a := range accounts {
			var count int
			if err := tx.DB().NewQuery("SELECT count(*) FROM hpahub_legacy_accounts WHERE legacy_id = {:id}").Bind(dbx.Params{"id": a.id}).Row(&count); err != nil { return err }
			if count != 0 { continue }
			record := core.NewRecord(collection)
			record.SetRandomPassword()
			record.Set("name", a.username)
			role := "member"
			if a.admin != 0 { role = "admin" }
			record.Set("role", role)
			avatar, err := legacyAvatar(a)
			if err != nil { return err }
			if avatar != nil { record.Set("avatar", avatar) }
			if err := tx.Save(record); err != nil { return fmt.Errorf("import legacy ID %d: %w", a.id, err) }
			if avatar != nil { uploaded = append(uploaded, record.BaseFilesPath()+"/"+record.GetString("avatar")) }
			_, err = tx.DB().NewQuery("INSERT INTO hpahub_legacy_accounts (legacy_id, user_id, username, password_hash) VALUES ({:id}, {:user}, {:username}, {:hash})").Bind(dbx.Params{"id": a.id, "user": record.Id, "username": a.username, "hash": a.hash}).Execute()
			if err != nil { return err }
		}
		return nil
	})
	if err != nil && len(uploaded) > 0 {
		fs, fsErr := app.NewFilesystem()
		if fsErr != nil { return errors.Join(err, fsErr) }
		defer fs.Close()
		for _, path := range uploaded { if deleteErr := fs.Delete(path); deleteErr != nil { err = errors.Join(err, deleteErr) } }
	}
	return err
}

func registerLegacyImportCommand(app core.App, root *cobra.Command) {
	var source string
	command := &cobra.Command{
		Use: "hpa-import-accounts",
		Short: "Import accounts from a read-only legacy SQLite snapshot",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RunAllMigrations(); err != nil { return err }
			return importLegacyAccounts(app, source)
		},
	}
	command.Flags().StringVar(&source, "source", "", "Path to the legacy users SQLite snapshot")
	command.MarkFlagRequired("source")
	root.AddCommand(command)
}
