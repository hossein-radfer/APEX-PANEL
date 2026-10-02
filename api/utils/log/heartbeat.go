package log

import (
	"runtime"
	"time"

	"go.uber.org/zap"
)

// StartResourceHeartbeat logs a snapshot of process resource usage on a
// fixed interval, for as long as the process is alive. This is the single
// most useful thing for diagnosing "the panel silently died and nobody
// knows why": an OOM kill or external SIGKILL gives the process no chance
// to log anything AT the moment it dies, so the only diagnostic signal
// available afterward is what the last few heartbeats before the gap
// showed. A steadily climbing HeapAlloc/Sys across heartbeats right up
// until the log stops points at a memory leak/OOM; goroutine count
// climbing without bound points at a leaked-goroutine bug; heartbeats
// continuing normally right up to a clean shutdown log line (or simply
// never resuming after a restart) points away from this process being at
// fault at all (an external kill, a host reboot, a supervisor issue).
//
// Runs in its own goroutine; the caller does not need to manage its
// lifecycle beyond keeping the process alive (it never returns).
func StartResourceHeartbeat(interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("recovered from panic in resource heartbeat logger", zap.Any("panic", r))
			}
		}()

		var lastNumGC uint32
		start := time.Now()

		for {
			time.Sleep(interval)
			logHeartbeat(start, &lastNumGC)
		}
	}()
}

func logHeartbeat(start time.Time, lastNumGC *uint32) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	gcSinceLast := m.NumGC - *lastNumGC
	*lastNumGC = m.NumGC

	zap.L().Info("resource heartbeat",
		zap.Duration("uptime", time.Since(start)),
		zap.Int("goroutines", runtime.NumGoroutine()),
		zap.Uint64("heap_alloc_mb", m.HeapAlloc/1024/1024),
		zap.Uint64("heap_sys_mb", m.HeapSys/1024/1024),
		zap.Uint64("sys_mb", m.Sys/1024/1024),
		zap.Uint32("gc_count_total", m.NumGC),
		zap.Uint32("gc_count_since_last_heartbeat", gcSinceLast),
	)
}
