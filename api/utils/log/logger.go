package log

import (
	"os"
	"path/filepath"

	"github.com/maahdima/mwp/api/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogFilePath returns the path InitLogger writes its rotating file log to,
// so other packages (the resource-heartbeat logger, the signal logger) can
// reference the same path without duplicating the join logic, and so it can
// be reported to the admin (e.g. in a future "download logs" endpoint).
func LogFilePath(cfg config.AppConfig) string {
	return filepath.Join(cfg.DataDirPath, "logs", "mwp.log")
}

// InitLogger writes to both stdout (so `mwp` run in a foreground terminal,
// or under a supervisor like systemd that captures stdout to journald,
// still shows logs live) AND a rotating file under DataDirPath/logs
// (so logs survive even when nothing is watching the terminal, and --
// critically -- so a crash that happens between someone checking the
// terminal and finding it dead later still leaves a trail on disk to
// diagnose what happened, rather than vanishing along with the process).
//
// The file sink logs at Info level (not just Warn+): diagnosing a silent
// death needs to see what was happening in the moments before it, not just
// error events -- an OOM kill or SIGKILL leaves no error entry at all (the
// process has no chance to log anything at the moment it dies), so the only
// way to narrow down the cause is to see the last thing the log recorded
// before it stops. Request logging (see http-server.go) and periodic
// resource heartbeats (see StartResourceHeartbeat) are both Info level
// specifically so they land here.
func InitLogger(cfg config.AppConfig) {
	logLevel := zap.WarnLevel
	if cfg.Mode == "development" {
		logLevel = zap.DebugLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var consoleEncoderCfg = encoderCfg
	consoleEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	var fileEncoderCfg = encoderCfg
	fileEncoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder // no ANSI color codes in the log file

	var consoleEncoder zapcore.Encoder
	if cfg.ConsoleLogFormat == "json" {
		consoleEncoder = zapcore.NewJSONEncoder(consoleEncoderCfg)
	} else {
		consoleEncoder = zapcore.NewConsoleEncoder(consoleEncoderCfg)
	}

	// The file sink is ALWAYS JSON, independent of ConsoleLogFormat -- the
	// in-panel Log Viewer (see api/service/log_reader.go) parses this file
	// line-by-line as structured JSON to power filtering/search/download.
	// The console/journald sink is unaffected and keeps whatever format the
	// operator configured for live tailing.
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderCfg)

	cores := []zapcore.Core{
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.NewAtomicLevelAt(logLevel)),
	}

	if cfg.DataDirPath != "" {
		logPath := LogFilePath(cfg)
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
			fileWriter := &lumberjack.Logger{
				Filename:   logPath,
				MaxSize:    50, // megabytes per file before rotating
				MaxBackups: 10,
				MaxAge:     7, // days -- rotated backups older than this are deleted automatically, so disk usage never grows unbounded
				Compress:   true,
			}
			cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), zap.NewAtomicLevelAt(zap.InfoLevel)))
		}
	}

	core := zapcore.NewTee(cores...)

	zap.ReplaceGlobals(zap.New(core, zap.AddCaller()))
}
