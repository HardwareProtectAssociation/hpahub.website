package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
)

// Walk directory descriptors instead of following symlinks in any path component.
func maintenanceOpenDirectory(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(strings.TrimPrefix(absolute, "/"), "/") {
		if name == "" {
			continue
		}
		next, openErr := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("unsafe or missing directory %s: %w", absolute, openErr)
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), absolute), nil
}

func maintenanceOpenChild(parent *os.File, name string, directory bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Openat(int(parent.Fd()), name, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if (!directory && !info.Mode().IsRegular()) || (directory && !info.IsDir()) {
		file.Close()
		return nil, fmt.Errorf("symlink or special file not allowed: %s", name)
	}
	return file, nil
}

func maintenanceFDPath(directory *os.File, name string) string {
	return fmt.Sprintf("/proc/self/fd/%d/%s", directory.Fd(), name)
}

func maintenanceBackupDatabase(source, destination string) error {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	parent, err := maintenanceOpenDirectory(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(absolute)
	file, err := maintenanceOpenChild(parent, name, false)
	if err != nil {
		return err
	}
	defer file.Close()
	// SQLite opens sidecars itself; reject symlinks and special files before opening it.
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		var info unix.Stat_t
		err := unix.Fstatat(int(parent.Fd()), name+suffix, &info, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("unsafe database sidecar: %s%s", source, suffix)
		}
	}
	uri := url.URL{Scheme: "file", Path: maintenanceFDPath(parent, name), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	connection, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer connection.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := output.Chmod(0600); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	// modernc exposes SQLite's online backup API via Conn.Raw. mode=ro keeps the
	// source read-only, while backup_step includes committed WAL pages consistently.
	err = connection.Raw(func(raw any) error {
		backupSource, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("sqlite driver does not expose online backup")
		}
		destinationURI := url.URL{Scheme: "file", Path: destination}
		backup, err := backupSource.NewBackup(destinationURI.String())
		if err != nil {
			return err
		}
		_, stepErr := backup.Step(-1)
		finishErr := backup.Finish()
		return errors.Join(stepErr, finishErr)
	})
	if err != nil {
		return err
	}
	return maintenanceCheckDatabase(destination, false)
}

func maintenanceCheckDatabase(path string, pocketbase bool) error {
	db, err := maintenanceOpenDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			rows.Close()
			return err
		}
		if result != "ok" {
			rows.Close()
			return fmt.Errorf("%s 完整性检查失败: %s", filepath.Base(path), result)
		}
		count++
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%s 完整性检查失败", filepath.Base(path))
	}
	if pocketbase {
		var exists int
		if err := db.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name='_collections'").Scan(&exists); err != nil {
			return fmt.Errorf("data.db 不是有效的 PocketBase 数据库: %w", err)
		}
	}
	return nil
}

func maintenanceCopyFile(parent *os.File, name, target, relative string) (maintenanceFile, error) {
	entry := maintenanceFile{Path: relative}
	source, err := maintenanceOpenChild(parent, name, false)
	if err != nil {
		return entry, err
	}
	defer source.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return entry, err
	}
	if err := output.Chmod(0600); err != nil {
		output.Close()
		return entry, err
	}
	digest := sha256.New()
	entry.Size, err = io.Copy(io.MultiWriter(output, digest), source)
	closeErr := output.Close()
	if err != nil || closeErr != nil {
		return entry, errors.Join(err, closeErr)
	}
	entry.SHA256 = hex.EncodeToString(digest.Sum(nil))
	return entry, nil
}

type maintenanceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func maintenanceMkdir(path string) error {
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	return os.Chmod(path, 0700)
}

func maintenanceCopyTree(source *os.File, target, relative string, entries *[]maintenanceFile) error {
	children, err := source.ReadDir(-1)
	if err != nil {
		return err
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	for _, child := range children {
		name := child.Name()
		path := filepath.Join(target, name)
		entryPath := relative + "/" + name
		if child.IsDir() {
			directory, err := maintenanceOpenChild(source, name, true)
			if err != nil {
				return err
			}
			if err := maintenanceMkdir(path); err != nil {
				directory.Close()
				return err
			}
			err = maintenanceCopyTree(directory, path, entryPath, entries)
			directory.Close()
			if err != nil {
				return err
			}
		} else {
			entry, err := maintenanceCopyFile(source, name, path, entryPath)
			if err != nil {
				return err
			}
			*entries = append(*entries, entry)
		}
	}
	return nil
}

func maintenanceSnapshot(gpu, warehouse, auth, output string) error {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	parent, err := maintenanceOpenDirectory(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(absolute)
	var existing unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &existing, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(err, unix.ENOENT) {
		if err != nil {
			return err
		}
		return fmt.Errorf("output already exists: %s", output)
	}
	root, err := maintenanceOpenDirectory(warehouse)
	if err != nil {
		return err
	}
	defer root.Close()
	projects, err := maintenanceOpenChild(root, "projects", true)
	if err != nil {
		return err
	}
	defer projects.Close()
	projectPath, err := filepath.Abs(filepath.Join(warehouse, "projects"))
	if err != nil {
		return err
	}
	if relative, err := filepath.Rel(projectPath, absolute); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("output cannot be inside warehouse projects")
	}
	temporary, err := os.MkdirTemp(maintenanceFDPath(parent, ""), "."+name+".")
	if err != nil {
		return err
	}
	defer func() {
		if temporary != "" {
			os.RemoveAll(temporary)
		}
	}()
	if err := os.Chmod(temporary, 0700); err != nil {
		return err
	}
	if err := maintenanceBackupDatabase(gpu, filepath.Join(temporary, "gpu.db")); err != nil {
		return fmt.Errorf("备份 GPU 数据库：%w", err)
	}
	if err := maintenanceBackupDatabase(auth, filepath.Join(temporary, "users.db")); err != nil {
		return fmt.Errorf("备份账户数据库：%w", err)
	}
	manifest, err := maintenanceReadManifest(temporary)
	if err != nil {
		return fmt.Errorf("读取快照清单：%w", err)
	}
	warehouseTarget := filepath.Join(temporary, "warehouse")
	if err := maintenanceMkdir(warehouseTarget); err != nil {
		return err
	}
	if err := maintenanceMkdir(filepath.Join(warehouseTarget, "projects")); err != nil {
		return err
	}
	if err := maintenanceCopyTree(projects, filepath.Join(warehouseTarget, "projects"), "projects", &manifest.WarehouseFiles); err != nil {
		return err
	}
	var info unix.Stat_t
	err = unix.Fstatat(int(root.Fd()), "storage_config.json", &info, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		entry, err := maintenanceCopyFile(root, "storage_config.json", filepath.Join(warehouseTarget, "storage_config.json"), "storage_config.json")
		if err != nil {
			return err
		}
		manifest.WarehouseFiles = append(manifest.WarehouseFiles, entry)
		manifest.StorageConfigPresent = true
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, "manifest.json"), append(data, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(temporary, "manifest.json"), 0600); err != nil {
		return err
	}
	// RENAME_NOREPLACE closes the existence-check race. Never use os.Rename here.
	if err := unix.Renameat2(int(parent.Fd()), filepath.Base(temporary), int(parent.Fd()), name, unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	temporary = ""
	return nil
}

type maintenanceManifest struct {
	Accounts struct {
		Count          int64            `json:"count"`
		HashAlgorithms map[string]int64 `json:"hash_algorithms"`
	} `json:"accounts"`
	GPUTables            map[string]int64  `json:"gpu_tables"`
	StorageConfigPresent bool              `json:"storage_config_present"`
	WarehouseFiles       []maintenanceFile `json:"warehouse_files"`
}

func maintenanceReadManifest(directory string) (maintenanceManifest, error) {
	manifest := maintenanceManifest{WarehouseFiles: []maintenanceFile{}}
	gpu, err := maintenanceOpenDB(filepath.Join(directory, "gpu.db"))
	if err != nil {
		return manifest, err
	}
	defer gpu.Close()
	manifest.GPUTables = map[string]int64{}
	rows, err := gpu.Query("SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name")
	if err != nil {
		return manifest, err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return manifest, err
		}
		tables = append(tables, name)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return manifest, err
	}
	for _, table := range tables {
		var count int64
		if err := gpu.QueryRow(`SELECT count(*) FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`).Scan(&count); err != nil {
			return manifest, err
		}
		manifest.GPUTables[table] = count
	}
	auth, err := maintenanceOpenDB(filepath.Join(directory, "users.db"))
	if err != nil {
		return manifest, err
	}
	defer auth.Close()
	manifest.Accounts.HashAlgorithms = map[string]int64{}
	rows, err = auth.Query("SELECT password_hash FROM users")
	if err != nil {
		return manifest, err
	}
	defer rows.Close()
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			return manifest, err
		}
		hash, ok := value.(string)
		if !ok || !strings.Contains(hash, "$") {
			return manifest, errors.New("invalid account password hash format")
		}
		algorithm, _, _ := strings.Cut(hash, "$")
		manifest.Accounts.HashAlgorithms[algorithm]++
		manifest.Accounts.Count++
	}
	return manifest, rows.Err()
}

func maintenanceValidateRestore(archive, destination string) error {
	root, err := maintenanceOpenDirectory(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := root.ReadDir(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) != 0 {
		return errors.New("解压目标必须是空目录")
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	seen := map[string]bool{}
	for _, entry := range reader.File {
		name := entry.Name
		parts := strings.Split(strings.TrimSuffix(name, "/"), "/")
		kind := (entry.ExternalAttrs >> 16) & unix.S_IFMT
		unsafe := name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\\x00") || parts[0] == "backups" || strings.HasPrefix(parts[0], ".local-restore-")
		for _, part := range parts {
			unsafe = unsafe || part == "" || part == "." || part == ".."
		}
		directory := strings.HasSuffix(name, "/")
		unsafe = unsafe || (kind != 0 && kind != unix.S_IFREG && kind != unix.S_IFDIR) || (directory && kind == unix.S_IFREG) || (!directory && kind == unix.S_IFDIR)
		if unsafe {
			return fmt.Errorf("不安全的备份路径或文件类型：%q", name)
		}
		path := strings.Join(parts, "/")
		if seen[path] {
			return fmt.Errorf("备份包含重复路径：%q", name)
		}
		seen[path] = true
	}
	for _, entry := range reader.File {
		if err := maintenanceExtractEntry(root, entry); err != nil {
			return err
		}
	}
	for _, name := range []string{"data.db", "auxiliary.db"} {
		file, err := maintenanceOpenChild(root, name, false)
		if name == "auxiliary.db" && errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			return fmt.Errorf("备份缺少或包含无效 %s: %w", name, err)
		}
		file.Close()
		if err := maintenanceCheckDatabase(maintenanceFDPath(root, name), name == "data.db"); err != nil {
			return err
		}
	}
	return nil
}

func maintenanceExtractEntry(root *os.File, entry *zip.File) error {
	parts := strings.Split(strings.TrimSuffix(entry.Name, "/"), "/")
	parentFD, err := unix.Dup(int(root.Fd()))
	if err != nil {
		return err
	}
	directory := strings.HasSuffix(entry.Name, "/")
	parents := parts[:len(parts)-1]
	if directory {
		parents = parts
	}
	for _, name := range parents {
		err := unix.Mkdirat(parentFD, name, 0700)
		if err != nil && !errors.Is(err, unix.EEXIST) {
			unix.Close(parentFD)
			return err
		}
		next, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(parentFD)
		if err != nil {
			return err
		}
		parentFD = next
	}
	defer unix.Close(parentFD)
	if directory {
		return nil
	}
	fd, err := unix.Openat(parentFD, parts[len(parts)-1], unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	output := os.NewFile(uintptr(fd), entry.Name)
	source, err := entry.Open()
	if err != nil {
		output.Close()
		return err
	}
	_, copyErr := io.Copy(output, source)
	return errors.Join(copyErr, source.Close(), output.Close())
}
