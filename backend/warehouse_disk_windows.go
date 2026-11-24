package main

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func warehouseDiskUsage(path string) map[string]uint64 {
	usage := map[string]uint64{"total": 0, "used": 0, "free": 0}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return usage
	}
	var available, total, free uint64
	if windows.GetDiskFreeSpaceEx(name, &available, &total, &free) != nil {
		return usage
	}
	usage["total"], usage["used"], usage["free"] = total, total-free, available
	return usage
}

func warehouseCreated(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, stat.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}
