package migrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func init() { migrations.Register(migrateLegacyAvatars, nil) }

func avatarConvert(input []byte, filter, codec, format string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-threads", "1", "-filter_threads", "1", "-max_pixels", "40000000", "-i", "pipe:0", "-frames:v", "1", "-vf", filter, "-c:v", codec, "-threads", "1", "-f", format, "pipe:1")
	cmd.Stdin = bytes.NewReader(input)
	result, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("avatar conversion failed: %w", err)
	}
	return result, nil
}

func migrateLegacyAvatars(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if users.Fields.GetByName("avatar_cropped") == nil {
		users.Fields.Add(&core.FileField{Name: "avatar_cropped", MaxSelect: 1, MaxSize: 5 << 20, MimeTypes: []string{"image/png"}, Protected: true})
		if err := app.Save(users); err != nil {
			return err
		}
	}
	storage, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer storage.Close()
	lastID := ""
	for {
		rows, err := app.FindRecordsByFilter("users", `id > {:id} && avatar != "" && avatar_thumb = ""`, "id", 50, 0, dbx.Params{"id": lastID})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			lastID = row.Id
			reader, err := storage.GetReader(row.BaseFilesPath() + "/" + row.GetString("avatar"))
			if err != nil {
				return fmt.Errorf("read avatar %s: %w", row.Id, err)
			}
			original, err := io.ReadAll(io.LimitReader(reader, 5<<20+1))
			reader.Close()
			if err != nil || len(original) > 5<<20 {
				return fmt.Errorf("read avatar %s: invalid or oversized file", row.Id)
			}
			cropped, err := avatarConvert(original, `crop=min(iw\,ih):min(iw\,ih),scale=min(512\,iw):min(512\,ih)`, "png", "image2pipe")
			if err != nil {
				return fmt.Errorf("crop avatar %s: %w", row.Id, err)
			}
			thumb, err := avatarConvert(cropped, "scale=144:144", "libwebp", "webp")
			if err != nil {
				return fmt.Errorf("thumbnail avatar %s: %w", row.Id, err)
			}
			fullFile, err := filesystem.NewFileFromBytes(cropped, "avatar.png")
			if err != nil {
				return err
			}
			thumbFile, err := filesystem.NewFileFromBytes(thumb, "avatar.webp")
			if err != nil {
				return err
			}
			row.Set("avatar_cropped", fullFile)
			row.Set("avatar_thumb", thumbFile)
			if err := app.Save(row); err != nil {
				return fmt.Errorf("save avatar %s: %w", row.Id, err)
			}
		}
	}
}
