package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
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
