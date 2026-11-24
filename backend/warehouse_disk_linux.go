package main

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func warehouseDiskUsage(path string) map[string]uint64 {
	usage := map[string]uint64{"total": 0, "used": 0, "free": 0}
	var stat unix.Statfs_t
	if unix.Statfs(path, &stat) != nil {
		return usage
	}
	block := uint64(stat.Frsize)
	usage["total"] = stat.Blocks * block
	usage["used"] = (stat.Blocks - stat.Bfree) * block
	usage["free"] = stat.Bavail * block
	return usage
}

func warehouseCreated(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	}
	return info.ModTime()
}
