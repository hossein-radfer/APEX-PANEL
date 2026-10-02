package log

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

// LogTerminationSignals logs SIGTERM/SIGINT the moment the process receives
// them, then re-raises the default behavior so the process still exits
// normally (this is a logging tap, not a shutdown handler). This is the
// other half of diagnosing a silent death alongside the resource
// heartbeat: SIGKILL (what an OOM killer sends) can NEVER be caught or
// logged by the receiving process -- that is a hard OS guarantee, not a gap
// in this code. So the diagnostic logic is:
//
//   - A log line here for SIGTERM/SIGINT right before the log stops -> a
//     supervisor or an operator asked the process to stop; expected.
//   - The log simply stops with NO termination signal logged, and the
//     last resource heartbeat (see heartbeat.go) shows normal/climbing
//     memory -> strongly suggests SIGKILL, i.e. almost certainly an OOM
//     kill (confirm with `dmesg`/`journalctl`, see deploy/README.md).
//   - The log simply stops with no termination signal AND the last
//     heartbeat looks completely normal (no memory growth, low goroutine
//     count) -> points away from this process/host resources entirely
//     (e.g. a hosting provider action, a hypervisor-level restart).
//
// Runs in its own goroutine; safe to call once at startup.
func LogTerminationSignals() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("recovered from panic in signal logger", zap.Any("panic", r))
			}
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		sig := <-sigCh
		zap.L().Warn("received termination signal; process is shutting down now", zap.String("signal", sig.String()))

		// Re-raise with the default handler so the process actually exits
		// (this goroutine only observes the signal, it doesn't own
		// shutdown behavior) -- stop intercepting first so the second
		// delivery isn't caught again.
		signal.Stop(sigCh)
		process, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = process.Signal(sig)
		}
	}()
}
