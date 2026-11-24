//go:build !linux

package main

import (
	"errors"
	"testing"
)

func TestMaintenanceUnsupportedPlatform(t *testing.T) {
	if err := maintenanceValidateRestore("unused.zip", t.TempDir()); !errors.Is(err, errMaintenanceUnsupported) {
		t.Fatalf("unsafe platform fallback: %v", err)
	}
}
