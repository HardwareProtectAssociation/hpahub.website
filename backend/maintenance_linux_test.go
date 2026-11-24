package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func maintenanceTestSources(t *testing.T) (root, gpu, auth, warehouse, output string) {
	t.Helper()
	root = t.TempDir()
	gpu, auth = filepath.Join(root, "gpu?#.db"), filepath.Join(root, "users.db")
	warehouse, output = filepath.Join(root, "warehouse"), filepath.Join(root, "snapshot")
	if err := os.MkdirAll(filepath.Join(warehouse, "projects", "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	// Leave this WAL connection open, so copying only the main file loses committed rows.
	maintenanceTestDB(t, gpu, "PRAGMA journal_mode=WAL", `CREATE TABLE "gpu table" (value TEXT)`, `INSERT INTO "gpu table" VALUES ('显卡')`)
	maintenanceTestDB(t, auth, "CREATE TABLE users (password_hash TEXT)", "INSERT INTO users VALUES ('scrypt:32768:8:1$salt$hash'), ('other$salt$hash')")
	return
}

func maintenanceLoadManifest(t *testing.T, output string) maintenanceManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest maintenanceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestMaintenanceSnapshotWALManifestPermissions(t *testing.T) {
	_, gpu, auth, warehouse, output := maintenanceTestSources(t)
	project := filepath.Join(warehouse, "projects", "项目")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(project, ".meta.json"):            "metadata",
		filepath.Join(project, "photo 1.png"):           "image",
		filepath.Join(warehouse, "storage_config.json"): "{}",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"hpa-legacy-snapshot", "--gpu-db", gpu, "--auth-db", auth, "--warehouse-root", warehouse, "--output", output}
	if handled, err := maintenanceCommand(args); !handled || err != nil {
		t.Fatalf("handled=%v error=%v", handled, err)
	}
	manifest := maintenanceLoadManifest(t, output)
	if manifest.GPUTables["gpu table"] != 1 || manifest.Accounts.Count != 2 || !reflect.DeepEqual(manifest.Accounts.HashAlgorithms, map[string]int64{"scrypt:32768:8:1": 1, "other": 1}) || !manifest.StorageConfigPresent {
		t.Fatalf("manifest: %+v", manifest)
	}
	if len(manifest.WarehouseFiles) != 3 {
		t.Fatalf("files: %+v", manifest.WarehouseFiles)
	}
	for _, entry := range manifest.WarehouseFiles {
		data, err := os.ReadFile(filepath.Join(output, "warehouse", filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if entry.Size != int64(len(data)) || entry.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("wrong digest: %+v", entry)
		}
	}
	if info, err := os.Stat(filepath.Join(output, "warehouse", "projects", "空目录")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory lost: %v", err)
	}
	db := maintenanceTestDB(t, filepath.Join(output, "gpu.db"))
	var value string
	if err := db.QueryRow(`SELECT value FROM "gpu table"`).Scan(&value); err != nil || value != "显卡" {
		t.Fatalf("WAL row lost: %q %v", value, err)
	}
	if err := filepath.WalkDir(output, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := os.FileMode(0600)
		if entry.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s permissions = %o, want %o", path, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenanceCommand(args); err == nil {
		t.Fatal("existing snapshot replaced")
	}
	if after := maintenanceLoadManifest(t, output); !reflect.DeepEqual(after, manifest) {
		t.Fatal("existing snapshot changed")
	}
}

func TestMaintenanceSnapshotEmptyProjects(t *testing.T) {
	_, gpu, auth, warehouse, output := maintenanceTestSources(t)
	if err := maintenanceSnapshot(gpu, warehouse, auth, output); err != nil {
		t.Fatal(err)
	}
	manifest := maintenanceLoadManifest(t, output)
	if manifest.StorageConfigPresent || manifest.WarehouseFiles == nil || len(manifest.WarehouseFiles) != 0 {
		t.Fatalf("manifest: %+v", manifest)
	}
}

func TestMaintenanceSnapshotRejectsUnsafeSources(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "config-symlink", "missing-projects", "invalid-db", "output-symlink", "source-symlink", "parent-symlink", "inside-projects", "bad-hash"} {
		t.Run(kind, func(t *testing.T) {
			root, gpu, auth, warehouse, output := maintenanceTestSources(t)
			switch kind {
			case "symlink":
				if err := os.Symlink(auth, filepath.Join(warehouse, "projects", "unsafe")); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(filepath.Join(warehouse, "projects", "unsafe"), 0600); err != nil {
					t.Fatal(err)
				}
			case "config-symlink":
				if err := os.Symlink(auth, filepath.Join(warehouse, "storage_config.json")); err != nil {
					t.Fatal(err)
				}
			case "missing-projects":
				if err := os.Rename(filepath.Join(warehouse, "projects"), filepath.Join(warehouse, "missing")); err != nil {
					t.Fatal(err)
				}
			case "invalid-db":
				if err := os.WriteFile(auth, []byte("not sqlite"), 0600); err != nil {
					t.Fatal(err)
				}
			case "output-symlink":
				if err := os.Symlink(filepath.Join(root, "missing"), output); err != nil {
					t.Fatal(err)
				}
			case "source-symlink":
				path := filepath.Join(root, "linked.db")
				if err := os.Symlink(auth, path); err != nil {
					t.Fatal(err)
				}
				auth = path
			case "parent-symlink":
				path := filepath.Join(root, "linked")
				if err := os.Symlink(warehouse, path); err != nil {
					t.Fatal(err)
				}
				warehouse = path
			case "inside-projects":
				output = filepath.Join(warehouse, "projects", "snapshot")
			case "bad-hash":
				maintenanceTestDB(t, auth, "UPDATE users SET password_hash=x'6162'")
			}
			if err := maintenanceSnapshot(gpu, warehouse, auth, output); err == nil {
				t.Fatal("unsafe snapshot accepted")
			}
			if kind == "output-symlink" {
				if info, err := os.Lstat(output); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("destination symlink changed")
				}
			} else if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatalf("partial output exists: %v", err)
			}
			matches, err := filepath.Glob(filepath.Join(root, ".snapshot.*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("staging directories leaked: %v %v", matches, err)
			}
		})
	}
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

func TestSnapshotContainerEntryRejectsUnsafeHostPaths(t *testing.T) {
	for _, kind := range []string{"inside-projects", "source-symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			gpu, auth := filepath.Join(root, "gpu.db"), filepath.Join(root, "users.db")
			warehouse := filepath.Join(root, "warehouse")
			if err := os.MkdirAll(filepath.Join(warehouse, "projects"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{gpu, auth} {
				if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(warehouse, "projects", "snapshot")
			want := "output cannot be inside warehouse projects"
			if kind == "source-symlink" {
				link := filepath.Join(root, "linked.db")
				if err := os.Symlink(gpu, link); err != nil {
					t.Fatal(err)
				}
				gpu, output, want = link, filepath.Join(root, "snapshot"), "symlink not allowed"
			}
			log := filepath.Join(root, "docker.log")
			if err := os.WriteFile(filepath.Join(root, "docker"), []byte("#!/bin/sh\nprintf touched > \"$DOCKER_TEST_LOG\"\nexit 99\n"), 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "sh", "../scripts/hpa-legacy-snapshot.sh", "--gpu-db", gpu, "--auth-db", auth, "--warehouse-root", warehouse+"/.", "--output", output)
			command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "DOCKER_TEST_LOG="+log)
			text, err := command.CombinedOutput()
			if err == nil || ctx.Err() != nil || !strings.Contains(string(text), want) {
				t.Fatalf("unsafe host path accepted: %s %v", text, err)
			}
			if _, err := os.Lstat(log); !os.IsNotExist(err) {
				t.Fatal("Docker invoked for unsafe host paths")
			}
		})
	}
}
