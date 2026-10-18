package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestUserManagerProtocolFile(t *testing.T) (*UserManagerProtocolFile, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:um_protocol_file_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.UserManagerProtocolConfig{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// Point the package-level storage dir at a fresh temp dir per test, so
	// tests never collide with each other or a real install's uploads.
	userManagerProtocolFilesPath = t.TempDir()

	return NewUserManagerProtocolFile(db), db
}

const (
	testServerA uint = 1
	testServerB uint = 2
)

// TestUploadFile_PreservesOriginalExtension is a regression guard for a
// confirmed, reported bug report's adjacent claim: uploaded protocol files
// must keep whatever extension the admin originally uploaded (.exe, .apk,
// .zip, .ovpn, etc.), never a hardcoded one.
func TestUploadFile_PreservesOriginalExtension(t *testing.T) {
	f, _ := newTestUserManagerProtocolFile(t)

	if err := f.UploadFile(model.ProtocolOpenVPN, testServerA, ProtocolFileKindClientApp, "MyVpnClient.exe", []byte("fake-binary")); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	path, err := f.GetFilePath(model.ProtocolOpenVPN, testServerA, ProtocolFileKindClientApp)
	if err != nil {
		t.Fatalf("failed to get file path: %v", err)
	}
	if got := DownloadFilename(model.ProtocolOpenVPN, ProtocolFileKindClientApp, path); got != "openvpn-client-app.exe" {
		t.Fatalf("expected filename to preserve .exe extension, got %q", got)
	}
}

// TestUploadFile_ReplacingWithDifferentExtensionRemovesOldFile confirms a
// second upload with a different extension doesn't leave the first file's
// bytes lingering on disk under its old name.
func TestUploadFile_ReplacingWithDifferentExtensionRemovesOldFile(t *testing.T) {
	f, _ := newTestUserManagerProtocolFile(t)

	if err := f.UploadFile(model.ProtocolSSTP, testServerA, ProtocolFileKindClientApp, "old.exe", []byte("old")); err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	firstPath, err := f.GetFilePath(model.ProtocolSSTP, testServerA, ProtocolFileKindClientApp)
	if err != nil {
		t.Fatalf("failed to get first file path: %v", err)
	}

	if err := f.UploadFile(model.ProtocolSSTP, testServerA, ProtocolFileKindClientApp, "new.apk", []byte("new")); err != nil {
		t.Fatalf("second upload failed: %v", err)
	}

	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("expected the old .exe file to be removed after re-upload with a new extension, stat err=%v", err)
	}

	secondPath, err := f.GetFilePath(model.ProtocolSSTP, testServerA, ProtocolFileKindClientApp)
	if err != nil {
		t.Fatalf("failed to get second file path: %v", err)
	}
	content, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("failed to read new file: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("expected new file content, got %q", string(content))
	}
}

// TestDeleteFile_RemovesFileAndClearsFlag is a regression test for a
// confirmed, reported gap: there was no way to delete an uploaded protocol
// file at all. Confirms DeleteFile removes the on-disk file and flips the
// has_*_file flag back to false.
func TestDeleteFile_RemovesFileAndClearsFlag(t *testing.T) {
	f, db := newTestUserManagerProtocolFile(t)

	if err := f.UploadFile(model.ProtocolL2TP, testServerA, ProtocolFileKindCertificate, "cert.pem", []byte("cert-bytes")); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	path, err := f.GetFilePath(model.ProtocolL2TP, testServerA, ProtocolFileKindCertificate)
	if err != nil {
		t.Fatalf("expected file to exist before delete: %v", err)
	}

	if err := f.DeleteFile(model.ProtocolL2TP, testServerA, ProtocolFileKindCertificate); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected the file to be removed from disk, stat err=%v", err)
	}

	if _, err := f.GetFilePath(model.ProtocolL2TP, testServerA, ProtocolFileKindCertificate); err == nil {
		t.Fatal("expected GetFilePath to fail after delete")
	}

	var cfg model.UserManagerProtocolConfig
	if err := db.Where("protocol = ? AND server_id = ?", model.ProtocolL2TP, testServerA).First(&cfg).Error; err != nil {
		t.Fatalf("failed to reload config row: %v", err)
	}
	if cfg.HasCertificateFile {
		t.Fatal("expected HasCertificateFile to be false after delete")
	}
}

// TestDeleteFile_NonExistentFileReturnsError confirms deleting a file that
// was never uploaded (or already deleted) reports a clean, expected error
// rather than silently succeeding or panicking.
func TestDeleteFile_NonExistentFileReturnsError(t *testing.T) {
	f, _ := newTestUserManagerProtocolFile(t)

	if err := f.DeleteFile(model.ProtocolPPTP, testServerA, ProtocolFileKindClientApp); err == nil {
		t.Fatal("expected an error deleting a file that was never uploaded")
	}
}

// TestDeleteFile_OnlyAffectsItsOwnKind confirms deleting the certificate
// file for a protocol doesn't touch that same protocol's client-app file
// (and vice versa) -- the two kinds must be fully independent.
func TestDeleteFile_OnlyAffectsItsOwnKind(t *testing.T) {
	f, _ := newTestUserManagerProtocolFile(t)

	if err := f.UploadFile(model.ProtocolIKEv2, testServerA, ProtocolFileKindCertificate, "cert.pem", []byte("cert")); err != nil {
		t.Fatalf("certificate upload failed: %v", err)
	}
	if err := f.UploadFile(model.ProtocolIKEv2, testServerA, ProtocolFileKindClientApp, "app.exe", []byte("app")); err != nil {
		t.Fatalf("client-app upload failed: %v", err)
	}

	if err := f.DeleteFile(model.ProtocolIKEv2, testServerA, ProtocolFileKindCertificate); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := f.GetFilePath(model.ProtocolIKEv2, testServerA, ProtocolFileKindCertificate); err == nil {
		t.Fatal("expected the certificate file to be gone")
	}
	if _, err := f.GetFilePath(model.ProtocolIKEv2, testServerA, ProtocolFileKindClientApp); err != nil {
		t.Fatalf("expected the client-app file to still exist, got error: %v", err)
	}
}

// TestUploadFile_IsolatedPerServer is a regression guard for the confirmed,
// reported multi-server bug this file's whole serverID parameter exists to
// fix: the same protocol configured on two different servers must each
// keep their own independent certificate/client-app file -- uploading or
// deleting one server's file must never affect the other server's file for
// the exact same protocol+kind.
func TestUploadFile_IsolatedPerServer(t *testing.T) {
	f, db := newTestUserManagerProtocolFile(t)

	if err := f.UploadFile(model.ProtocolOpenVPN, testServerA, ProtocolFileKindCertificate, "cert-a.pem", []byte("server-a-cert")); err != nil {
		t.Fatalf("server A upload failed: %v", err)
	}
	if err := f.UploadFile(model.ProtocolOpenVPN, testServerB, ProtocolFileKindCertificate, "cert-b.pem", []byte("server-b-cert")); err != nil {
		t.Fatalf("server B upload failed: %v", err)
	}

	pathA, err := f.GetFilePath(model.ProtocolOpenVPN, testServerA, ProtocolFileKindCertificate)
	if err != nil {
		t.Fatalf("failed to get server A file path: %v", err)
	}
	pathB, err := f.GetFilePath(model.ProtocolOpenVPN, testServerB, ProtocolFileKindCertificate)
	if err != nil {
		t.Fatalf("failed to get server B file path: %v", err)
	}
	if pathA == pathB {
		t.Fatalf("expected server A and B to have distinct on-disk paths, both resolved to %q", pathA)
	}

	contentA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("failed to read server A file: %v", err)
	}
	if string(contentA) != "server-a-cert" {
		t.Fatalf("expected server A's own content, got %q", string(contentA))
	}

	// Deleting server A's file must leave server B's file (same protocol,
	// same kind, different server) completely untouched.
	if err := f.DeleteFile(model.ProtocolOpenVPN, testServerA, ProtocolFileKindCertificate); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := f.GetFilePath(model.ProtocolOpenVPN, testServerA, ProtocolFileKindCertificate); err == nil {
		t.Fatal("expected server A's file to be gone after its own delete")
	}
	if _, err := f.GetFilePath(model.ProtocolOpenVPN, testServerB, ProtocolFileKindCertificate); err != nil {
		t.Fatalf("expected server B's file to be unaffected by server A's delete, got error: %v", err)
	}

	var cfgA, cfgB model.UserManagerProtocolConfig
	if err := db.Where("protocol = ? AND server_id = ?", model.ProtocolOpenVPN, testServerA).First(&cfgA).Error; err != nil {
		t.Fatalf("failed to reload server A config row: %v", err)
	}
	if cfgA.HasCertificateFile {
		t.Fatal("expected server A's HasCertificateFile to be false after delete")
	}
	if err := db.Where("protocol = ? AND server_id = ?", model.ProtocolOpenVPN, testServerB).First(&cfgB).Error; err != nil {
		t.Fatalf("failed to reload server B config row: %v", err)
	}
	if !cfgB.HasCertificateFile {
		t.Fatal("expected server B's HasCertificateFile to remain true after server A's delete")
	}
}
