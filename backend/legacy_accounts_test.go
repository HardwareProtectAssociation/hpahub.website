package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"golang.org/x/crypto/scrypt"
)

func TestLegacyAccountImport(t *testing.T) {
	app := invitationTestApp(t)
	path := filepath.Join(t.TempDir(), "users.db")
	db, err := sql.Open("sqlite", path)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER, username TEXT, password_hash TEXT, is_admin INTEGER, avatar BLOB, avatar_mime TEXT)`); err != nil { t.Fatal(err) }
	key, err := scrypt.Key([]byte(strings.Repeat("密码", 40)), []byte("fixed-salt"), 32768, 8, 1, 64)
	if err != nil { t.Fatal(err) }
	hash := "scrypt:32768:8:1$fixed-salt$" + hex.EncodeToString(key)
	var avatar bytes.Buffer
	if err := png.Encode(&avatar, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil { t.Fatal(err) }
	if _, err := db.Exec("INSERT INTO users VALUES (1, ?, ?, 1, ?, ?), (2, ?, ?, 0, ?, ?)", "旧管理员", hash, avatar.Bytes(), "image/png", "member", hash, []byte("<svg/>"), "image/svg+xml"); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err != nil { t.Fatal(err) }
	var id string
	if err := app.DB().NewQuery("SELECT user_id FROM hpahub_legacy_accounts WHERE legacy_id = 1").Row(&id); err != nil { t.Fatal(err) }
	user, err := app.FindRecordById("users", id)
	if err != nil { t.Fatal(err) }
	if user.GetString("name") != "旧管理员" || user.GetString("role") != "admin" || user.Email() != "" || user.GetString("avatar") == "" || user.ValidatePassword(strings.Repeat("密码", 40)) { t.Fatal("imported admin fields or native password are invalid") }
	var memberID string
	if err := app.DB().NewQuery("SELECT user_id FROM hpahub_legacy_accounts WHERE legacy_id = 2").Row(&memberID); err != nil { t.Fatal(err) }
	member, err := app.FindRecordById("users", memberID)
	if err != nil { t.Fatal(err) }
	if member.GetString("role") != "member" || member.GetString("avatar") != "" { t.Fatal("member role or unsafe avatar handling is invalid") }
	user.Set("name", "Changed")
	user.Set("role", "member")
	user.SetPassword("changed-password-123")
	if err := app.Save(user); err != nil { t.Fatal(err) }
	if _, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = 1", "scrypt:32768:8:1$other$"+strings.Repeat("a", 128)); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err != nil { t.Fatal(err) }
	user, err = app.FindRecordById("users", id)
	if err != nil { t.Fatal(err) }
	var storedHash string
	if err := app.DB().NewQuery("SELECT password_hash FROM hpahub_legacy_accounts WHERE legacy_id = 1").Row(&storedHash); err != nil { t.Fatal(err) }
	if user.GetString("name") != "Changed" || user.GetString("role") != "member" || !user.ValidatePassword("changed-password-123") || storedHash != hash { t.Fatal("repeat import overwrote existing account") }

	if _, err := db.Exec("INSERT INTO users VALUES (3, 'new', ?, 0, NULL, NULL), (4, 'broken', 'unsupported', 0, NULL, NULL)", hash); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err == nil { t.Fatal("unsupported hash accepted") }
	var count int
	if err := app.DB().NewQuery("SELECT count(*) FROM hpahub_legacy_accounts WHERE legacy_id = 3").Row(&count); err != nil || count != 0 { t.Fatalf("partial import: count=%d err=%v", count, err) }
	if _, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = 4", hash); err != nil { t.Fatal(err) }
	if _, err := db.Exec("UPDATE users SET username = 'changed-owner' WHERE id = 1"); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err == nil { t.Fatal("ownership mismatch accepted") }
	if err := app.DB().NewQuery("SELECT count(*) FROM hpahub_legacy_accounts WHERE legacy_id = 3").Row(&count); err != nil || count != 0 { t.Fatalf("partial import: count=%d err=%v", count, err) }
	if _, err := db.Exec("UPDATE users SET username = '旧管理员' WHERE id = 1"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("UPDATE users SET username = 'member' WHERE id = 4"); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err == nil { t.Fatal("duplicate username accepted") }
	if _, err := db.Exec("UPDATE users SET username = 'fourth' WHERE id = 4; UPDATE users SET id = 5 WHERE id = 2"); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err == nil { t.Fatal("existing username conflict accepted") }
	if _, err := db.Exec("UPDATE users SET id = 2 WHERE id = 5"); err != nil { t.Fatal(err) }
	if _, err := app.DB().NewQuery("CREATE TRIGGER reject_legacy_four BEFORE INSERT ON hpahub_legacy_accounts WHEN NEW.legacy_id = 4 BEGIN SELECT RAISE(ABORT, 'reject'); END").Execute(); err != nil { t.Fatal(err) }
	if err := importLegacyAccounts(app, path); err == nil { t.Fatal("database failure accepted") }
	if err := app.DB().NewQuery("SELECT count(*) FROM hpahub_legacy_accounts WHERE legacy_id = 3").Row(&count); err != nil || count != 0 { t.Fatalf("transaction did not roll back: count=%d err=%v", count, err) }
}
