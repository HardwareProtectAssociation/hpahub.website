package main

import (
	"testing"
	"time"
)

func TestBackupPolicy(t *testing.T) {
	for _, tc := range []struct {
		policy policy
		valid  bool
	}{
		{policy{IntervalHours: 0, RetainCount: 1}, true},
		{policy{IntervalHours: 8760, RetainCount: 100}, true},
		{policy{IntervalHours: -1, RetainCount: 1}, false},
		{policy{IntervalHours: 8761, RetainCount: 1}, false},
		{policy{IntervalHours: 1, RetainCount: 0}, false},
		{policy{IntervalHours: 1, RetainCount: 101}, false},
	} {
		if got := tc.policy.valid(); got != tc.valid { t.Errorf("valid(%+v) = %v", tc.policy, got) }
	}
	now := time.Unix(7200, 0)
	if (policy{IntervalHours: 0, LastRun: 0}).due(now) { t.Fatal("disabled policy is due") }
	if (policy{IntervalHours: 2, LastRun: 1}).due(now) { t.Fatal("policy is due too early") }
	if !(policy{IntervalHours: 2, LastRun: 0}).due(now) { t.Fatal("policy is not due at interval") }
}
