package service

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestSplitFileForTelegram_SmallFileIsNotSplit confirms the common-case
// no-op: a file already under TelegramMaxDocumentBytes comes back as a
// single-element slice pointing at the ORIGINAL path, with no part files
// created on disk.
func TestSplitFileForTelegram_SmallFileIsNotSplit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small-backup.db")
	if err := os.WriteFile(path, []byte("not actually a real backup, just small"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	svc := &BackupService{}
	parts, cleanup, err := svc.SplitFileForTelegram(path)
	if err != nil {
		t.Fatalf("SplitFileForTelegram failed: %v", err)
	}
	defer cleanup()

	if len(parts) != 1 || parts[0] != path {
		t.Fatalf("expected a single-element slice pointing at the original path, got %v", parts)
	}
}

// TestSplitFileForTelegram_LargeFileSplitsIntoReassemblableParts confirms
// the real fix: a file over TelegramMaxDocumentBytes is split into multiple
// parts, each within the per-chunk target, and concatenating every part
// back together byte-for-byte reproduces the original file exactly --
// proving the admin's own `cat *.part* > restored` reassembly instructions
// (see BotScheduler.broadcastDocumentToAdmins) actually work.
func TestSplitFileForTelegram_LargeFileSplitsIntoReassemblableParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large-backup.db")

	// Slightly over 2x TelegramMaxDocumentBytes, using random bytes (not
	// zero-filled) so a byte-for-byte reassembly comparison is a real test,
	// not something a naive all-zeros bug could pass by accident.
	size := TelegramMaxDocumentBytes*2 + 1024
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		t.Fatalf("failed to generate random content: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write large test file: %v", err)
	}

	svc := &BackupService{}
	parts, cleanup, err := svc.SplitFileForTelegram(path)
	if err != nil {
		t.Fatalf("SplitFileForTelegram failed: %v", err)
	}
	defer cleanup()

	if len(parts) < 2 {
		t.Fatalf("expected at least 2 parts for a file just over 2x the limit, got %d", len(parts))
	}

	var reassembled bytes.Buffer
	for i, part := range parts {
		info, statErr := os.Stat(part)
		if statErr != nil {
			t.Fatalf("failed to stat part %d (%s): %v", i, part, statErr)
		}
		if info.Size() > telegramChunkTargetBytes {
			t.Fatalf("part %d (%s) is %d bytes, exceeds telegramChunkTargetBytes (%d)", i, part, info.Size(), telegramChunkTargetBytes)
		}
		data, readErr := os.ReadFile(part)
		if readErr != nil {
			t.Fatalf("failed to read part %d: %v", i, readErr)
		}
		reassembled.Write(data)
	}

	if !bytes.Equal(reassembled.Bytes(), content) {
		t.Fatal("reassembled parts do not byte-for-byte match the original file content")
	}
}

// TestSplitFileForTelegram_CleanupRemovesAllParts confirms the returned
// cleanup func actually removes every part file it created, not just the
// first/last one.
func TestSplitFileForTelegram_CleanupRemovesAllParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cleanup-backup.db")

	content := make([]byte, TelegramMaxDocumentBytes+1)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	svc := &BackupService{}
	parts, cleanup, err := svc.SplitFileForTelegram(path)
	if err != nil {
		t.Fatalf("SplitFileForTelegram failed: %v", err)
	}
	if len(parts) < 2 {
		t.Fatalf("expected at least 2 parts, got %d", len(parts))
	}

	cleanup()

	for _, part := range parts {
		if _, statErr := os.Stat(part); !os.IsNotExist(statErr) {
			t.Fatalf("expected part %s to be removed by cleanup, stat error: %v", part, statErr)
		}
	}
}
