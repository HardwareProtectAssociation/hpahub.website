package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func maintenanceTestDB(t *testing.T, path string, statements ...string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func maintenanceTestZIP(t *testing.T, archive string, names []string, modes []os.FileMode, contents [][]byte) {
	t.Helper()
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for i, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if modes != nil {
			header.SetMode(modes[i])
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if contents != nil {
			if _, err := entry.Write(contents[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceRestoreValidation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.db")
	db := maintenanceTestDB(t, source, "CREATE TABLE _collections (id TEXT)", "INSERT INTO _collections VALUES ('test')")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "backup.zip")
	maintenanceTestZIP(t, archive, []string{"data.db", "auxiliary.db", "uploads/空目录/", "uploads/照片.txt"}, nil, [][]byte{data, data, nil, []byte("photo")})
	destination := filepath.Join(root, "restore")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if handled, err := maintenanceCommand([]string{"hpa-validate-restore", archive, destination}); !handled || err != nil {
		t.Fatalf("handled=%v error=%v", handled, err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "uploads", "照片.txt"))
	if err != nil || !bytes.Equal(got, []byte("photo")) {
		t.Fatalf("file not extracted: %q %v", got, err)
	}
	if err := maintenanceValidateRestore(archive, destination); err == nil {
		t.Fatal("nonempty destination accepted")
	}
}

func TestMaintenanceRestoreRejectsUnsafeArchives(t *testing.T) {
	for _, test := range []struct {
		name     string
		names    []string
		modes    []os.FileMode
		contents [][]byte
	}{
		{"traversal", []string{"../escaped"}, nil, nil},
		{"absolute", []string{"/escaped"}, nil, nil},
		{"backslash", []string{`a\b`}, nil, nil},
		{"dot", []string{"a/./b"}, nil, nil},
		{"empty-component", []string{"a//b"}, nil, nil},
		{"backups", []string{"backups/file"}, nil, nil},
		{"stage", []string{".local-restore-stage-123/file"}, nil, nil},
		{"duplicate", []string{"data.db", "data.db"}, nil, nil},
		{"directory-duplicate", []string{"a/", "a/"}, nil, nil},
		{"symlink", []string{"data.db"}, []os.FileMode{os.ModeSymlink | 0600}, nil},
		{"fifo", []string{"data.db"}, []os.FileMode{os.ModeNamedPipe | 0600}, nil},
		{"type-conflict", []string{"data.db", "data.db/file"}, nil, nil},
		{"missing-db", []string{"uploads/file"}, nil, nil},
		{"invalid-db", []string{"data.db"}, nil, [][]byte{[]byte("not sqlite")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "backup.zip")
			maintenanceTestZIP(t, archive, test.names, test.modes, test.contents)
			destination := filepath.Join(root, "restore")
			if err := os.Mkdir(destination, 0700); err != nil {
				t.Fatal(err)
			}
			if err := maintenanceValidateRestore(archive, destination); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, "escaped")); !os.IsNotExist(err) {
				t.Fatal("archive escaped destination")
			}
		})
	}
}

func TestMaintenanceRestoreRequiresPocketBaseDatabase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.db")
	db := maintenanceTestDB(t, source, "CREATE TABLE unrelated (id TEXT)")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "backup.zip")
	maintenanceTestZIP(t, archive, []string{"data.db"}, nil, [][]byte{data})
	destination := filepath.Join(root, "restore")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenanceValidateRestore(archive, destination); err == nil || !strings.Contains(err.Error(), "PocketBase") {
		t.Fatalf("non-PocketBase database accepted: %v", err)
	}
}

func TestLocalRestoreValidatesBeforeConfirmationOrStop(t *testing.T) {
	root := t.TempDir()
	docker := filepath.Join(root, "docker")
	log := filepath.Join(root, "docker.log")
	// Only fake Docker is reachable. A failed validator must abort before asking
	// for confirmation, stopping Compose, or touching any Docker volume.
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$DOCKER_TEST_LOG\"\ncase \"$*\" in *hpa-validate-restore*) echo '不安全的备份路径' >&2; exit 42 ;; esac\n"
	if err := os.WriteFile(docker, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(masterFD)
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", "../scripts/local-restore.sh", filepath.Join(root, "unsafe.zip"))
	command.Stdin = terminal
	command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "DOCKER_TEST_LOG="+log)
	output, err := command.CombinedOutput()
	if err == nil || ctx.Err() != nil {
		t.Fatalf("restore failed to abort: %s %v", output, err)
	}
	if !strings.Contains(string(output), "不安全") || strings.Contains(string(output), "RESTORE") {
		t.Fatalf("validation order changed: %s", output)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "--network none") || !strings.Contains(string(calls), "--entrypoint /app/pocketbase") || !strings.Contains(string(calls), "dst=/backup.zip,readonly") || !strings.Contains(string(calls), "hpa-validate-restore /backup.zip /restore") || strings.Contains(string(calls), "compose") || strings.Contains(string(calls), "volume") {
		t.Fatalf("restore touched services before successful validation: %s", calls)
	}
}
