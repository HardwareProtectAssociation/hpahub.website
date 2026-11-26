package migrations

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	_ "golang.org/x/image/webp"
)

func TestLegacyAvatarMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(users)
	r.SetEmail("legacy@example.com")
	r.SetPassword("password123456")
	img := image.NewRGBA(image.Rect(0, 0, 800, 400))
	img.Set(400, 200, color.RGBA{255, 0, 0, 255})
	var input bytes.Buffer
	if err := png.Encode(&input, img); err != nil {
		t.Fatal(err)
	}
	file, err := filesystem.NewFileFromBytes(input.Bytes(), "legacy.png")
	if err != nil {
		t.Fatal(err)
	}
	r.Set("avatar", file)
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
	old := r.GetString("avatar")
	if err := migrateLegacyAvatars(app); err != nil {
		t.Fatal(err)
	}
	r, err = app.FindRecordById("users", r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetString("avatar") != old || r.GetString("avatar_cropped") == "" || r.GetString("avatar_thumb") == "" {
		t.Fatalf("missing files: %v", r.PublicExport())
	}
	fs, err := app.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	original, err := fs.GetReader(r.BaseFilesPath() + "/" + old)
	if err != nil {
		t.Fatal(err)
	}
	originalData, err := io.ReadAll(original)
	original.Close()
	if err != nil || !bytes.Equal(originalData, input.Bytes()) {
		t.Fatal("original avatar changed")
	}
	for field, size := range map[string]int{"avatar_cropped": 400, "avatar_thumb": 144} {
		reader, err := fs.GetReader(r.BaseFilesPath() + "/" + r.GetString(field))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Fatalf("%s = %dx%d: %v", field, cfg.Width, cfg.Height, err)
		}
	}
	thumb := r.GetString("avatar_thumb")
	if err := migrateLegacyAvatars(app); err != nil {
		t.Fatal(err)
	}
	r, err = app.FindRecordById("users", r.Id)
	if err != nil || r.GetString("avatar_thumb") != thumb {
		t.Fatal("migration was not idempotent")
	}
}

func TestLegacyAvatarConversionFailureAndSize(t *testing.T) {
	if _, err := avatarConvert([]byte("not an image"), "scale=144:144", "libwebp", "webp"); err == nil {
		t.Fatal("invalid image accepted")
	}
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 900, 700))); err != nil {
		t.Fatal(err)
	}
	data, err := avatarConvert(input.Bytes(), `crop=min(iw\,ih):min(iw\,ih),scale=min(512\,iw):min(512\,ih)`, "png", "image2pipe")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 512 || cfg.Height != 512 {
		t.Fatalf("crop = %dx%d: %v", cfg.Width, cfg.Height, err)
	}
}

func TestLegacyAvatarMigrationRollsBackOnMissingFile(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range []string{"aaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbb"} {
		r := core.NewRecord(users)
		r.Id = id
		r.SetEmail(id + "@example.com")
		r.SetPassword("password123456")
		file, err := filesystem.NewFileFromBytes(input.Bytes(), "legacy.png")
		if err != nil {
			t.Fatal(err)
		}
		r.Set("avatar", file)
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.Id)
	}
	missing, err := app.FindRecordById("users", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	fs, err := app.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	if err := fs.Delete(missing.BaseFilesPath() + "/" + missing.GetString("avatar")); err != nil {
		t.Fatal(err)
	}
	if err := app.RunInTransaction(migrateLegacyAvatars); err == nil {
		t.Fatal("expected missing file error")
	}
	first, err := app.FindRecordById("users", ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if first.GetString("avatar") == "" || first.GetString("avatar_thumb") != "" || first.GetString("avatar_cropped") != "" {
		t.Fatal("migration did not roll back")
	}
}
