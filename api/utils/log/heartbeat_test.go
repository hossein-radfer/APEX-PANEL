package log

import (
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogHeartbeatEmitsExpectedFields(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	var lastNumGC uint32
	logHeartbeat(time.Now().Add(-time.Minute), &lastNumGC)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "resource heartbeat" {
		t.Fatalf("expected message %q, got %q", "resource heartbeat", entry.Message)
	}

	fields := entry.ContextMap()
	for _, key := range []string{"uptime", "goroutines", "heap_alloc_mb", "heap_sys_mb", "sys_mb", "gc_count_total", "gc_count_since_last_heartbeat"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("expected field %q in heartbeat log entry, not found in %v", key, fields)
		}
	}
}
