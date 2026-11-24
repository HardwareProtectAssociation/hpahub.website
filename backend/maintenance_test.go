package main

import (
	"testing"
)

func TestMaintenanceCommandDispatch(t *testing.T) {
	for _, args := range [][]string{nil, {"serve"}, {"hpa-import-accounts"}} {
		if handled, err := maintenanceCommand(args); handled || err != nil {
			t.Fatalf("unexpected maintenance dispatch: %v %v %v", args, handled, err)
		}
	}
	for _, args := range [][]string{{"hpa-validate-restore"}, {"hpa-validate-restore", "a", "b", "extra"}, {"hpa-legacy-snapshot"}, {"hpa-legacy-snapshot", "--unknown"}} {
		if handled, err := maintenanceCommand(args); !handled || err == nil {
			t.Fatalf("invalid arguments accepted: %v %v %v", args, handled, err)
		}
	}
}
