package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openSecurityRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:security_retention_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Peer{}, &model.UserManagerAccount{}, &model.IPConnectionLog{}, &model.EtherTrafficSample{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestDeleteConnectionLogForInactiveUsers_ActivePeerKept confirms a peer
// that is NOT disabled keeps its IPConnectionLog row.
func TestDeleteConnectionLogForInactiveUsers_ActivePeerKept(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	activePeer := model.Peer{UUID: "uuid-p1", Name: "active", PeerID: "p1", Interface: "wg1", AllowedAddress: "10.0.0.1/32", Disabled: false}
	if err := db.Create(&activePeer).Error; err != nil {
		t.Fatalf("failed to create active peer: %v", err)
	}

	if err := db.Create(&model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "active", PeerID: &activePeer.ID,
		IPAddress: "1.1.1.1", ConnectedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to create connection log: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteConnectionLogForInactiveUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 rows deleted for an active peer, got %d", deleted)
	}

	var count int64
	db.Model(&model.IPConnectionLog{}).Count(&count)
	if count != 1 {
		t.Errorf("expected the active peer's connection log row to survive, got %d rows remaining", count)
	}
}

// TestDeleteConnectionLogForInactiveUsers_DisabledPeerDeleted confirms a
// disabled peer's IPConnectionLog row IS deleted.
func TestDeleteConnectionLogForInactiveUsers_DisabledPeerDeleted(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	disabledPeer := model.Peer{UUID: "uuid-p2", Name: "gone", PeerID: "p2", Interface: "wg1", AllowedAddress: "10.0.0.2/32", Disabled: true}
	if err := db.Create(&disabledPeer).Error; err != nil {
		t.Fatalf("failed to create disabled peer: %v", err)
	}

	if err := db.Create(&model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "gone", PeerID: &disabledPeer.ID,
		IPAddress: "1.1.1.2", ConnectedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to create connection log: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteConnectionLogForInactiveUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 row deleted for a disabled peer, got %d", deleted)
	}
}

// TestDeleteConnectionLogForInactiveUsers_DeletedPeerRowsDeleted confirms
// a WireGuard peer that no longer exists in the peers table AT ALL (not
// merely disabled -- deleted outright) also has its orphaned
// IPConnectionLog row cleaned up. This exercises the "PeerID NOT IN
// activePeerIDs" branch when the peer row is entirely gone, not just
// zero active peers overall.
func TestDeleteConnectionLogForInactiveUsers_DeletedPeerRowsDeleted(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	// One still-active peer exists (so activePeerIDs is non-empty), and one
	// orphaned row references a peer ID that was never created at all.
	activePeer := model.Peer{UUID: "uuid-p3", Name: "active", PeerID: "p3", Interface: "wg1", AllowedAddress: "10.0.0.3/32", Disabled: false}
	if err := db.Create(&activePeer).Error; err != nil {
		t.Fatalf("failed to create active peer: %v", err)
	}
	orphanedID := activePeer.ID + 999
	if err := db.Create(&model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "deleted-peer", PeerID: &orphanedID,
		IPAddress: "1.1.1.3", ConnectedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to create orphaned connection log: %v", err)
	}
	if err := db.Create(&model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "active", PeerID: &activePeer.ID,
		IPAddress: "1.1.1.4", ConnectedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to create active connection log: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteConnectionLogForInactiveUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected exactly 1 orphaned row deleted, got %d", deleted)
	}

	var remaining model.IPConnectionLog
	if err := db.First(&remaining).Error; err != nil {
		t.Fatalf("expected the active peer's row to remain: %v", err)
	}
	if remaining.Identity != "active" {
		t.Errorf("expected the surviving row to belong to 'active', got %q", remaining.Identity)
	}
}

// TestDeleteConnectionLogForInactiveUsers_NoActivePeersDeletesAll pins down
// the edge case reasoned through during implementation: when there are ZERO
// active peers at all (activePeerIDs is empty), the WHERE clause must fall
// back to deleting every wireguard row unconditionally, NOT silently keep
// everything because an empty `NOT IN ()` was skipped incorrectly.
func TestDeleteConnectionLogForInactiveUsers_NoActivePeersDeletesAll(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	disabledPeer := model.Peer{UUID: "uuid-p4", Name: "only-disabled", PeerID: "p4", Interface: "wg1", AllowedAddress: "10.0.0.4/32", Disabled: true}
	if err := db.Create(&disabledPeer).Error; err != nil {
		t.Fatalf("failed to create disabled peer: %v", err)
	}
	if err := db.Create(&model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "only-disabled", PeerID: &disabledPeer.ID,
		IPAddress: "1.1.1.5", ConnectedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to create connection log: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteConnectionLogForInactiveUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected the only row (belonging to the only, disabled peer) to be deleted, got %d deleted", deleted)
	}
}

// TestDeleteConnectionLogOlderThan confirms the age-based cutoff only
// removes rows strictly before the cutoff.
func TestDeleteConnectionLogOlderThan(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	old := model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "old", IPAddress: "1.1.1.1",
		ConnectedAt: time.Now().AddDate(0, 0, -10),
	}
	recent := model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "recent", IPAddress: "1.1.1.2",
		ConnectedAt: time.Now(),
	}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old row: %v", err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("failed to create recent row: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteConnectionLogOlderThan(time.Now().AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected exactly 1 row (the 10-day-old one) deleted, got %d", deleted)
	}

	var remaining model.IPConnectionLog
	if err := db.First(&remaining).Error; err != nil {
		t.Fatalf("expected the recent row to remain: %v", err)
	}
	if remaining.Identity != "recent" {
		t.Errorf("expected the surviving row to be 'recent', got %q", remaining.Identity)
	}
}

// TestDeleteConnectionLogOlderThan_IsHardDelete is the regression test for
// a confirmed, reported bug: IPConnectionLog embeds model.Model (carrying
// gorm.DeletedAt), so a plain .Delete() call only soft-deleted rows here,
// defeating this entire service's purpose (freeing real disk space). An
// ordinary db.First() query (used by the test above) can't tell a
// soft-delete from a hard-delete since GORM auto-filters deleted_at --
// this test uses Unscoped() explicitly to prove the row is genuinely gone.
func TestDeleteConnectionLogOlderThan_IsHardDelete(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	old := model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "old", IPAddress: "1.1.1.1",
		ConnectedAt: time.Now().AddDate(0, 0, -10),
	}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old row: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	if _, err := svc.DeleteConnectionLogOlderThan(time.Now().AddDate(0, 0, -7)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var unscopedCount int64
	if err := db.Unscoped().Model(&model.IPConnectionLog{}).Where("id = ?", old.ID).Count(&unscopedCount).Error; err != nil {
		t.Fatalf("failed to count unscoped: %v", err)
	}
	if unscopedCount != 0 {
		t.Fatalf("expected the old row to be genuinely (hard) deleted, but it's still present when queried Unscoped()")
	}
}

// TestRunScheduledCleanup_DeletesOnlyRowsOlderThanDefaultRetention
// confirms the nightly automatic job applies defaultRetentionDays to BOTH
// IPConnectionLog and EtherTrafficSample, leaving recent rows of each
// untouched.
func TestRunScheduledCleanup_DeletesOnlyRowsOlderThanDefaultRetention(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	oldLog := model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "old-log", IPAddress: "1.1.1.1",
		ConnectedAt: time.Now().AddDate(0, 0, -(defaultRetentionDays + 1)),
	}
	recentLog := model.IPConnectionLog{
		Protocol: model.UsageProtocolWireGuard, Identity: "recent-log", IPAddress: "1.1.1.2",
		ConnectedAt: time.Now(),
	}
	if err := db.Create(&oldLog).Error; err != nil {
		t.Fatalf("failed to create old log: %v", err)
	}
	if err := db.Create(&recentLog).Error; err != nil {
		t.Fatalf("failed to create recent log: %v", err)
	}

	oldSample := model.EtherTrafficSample{
		Interface: "ether1", SrcAddress: "10.0.0.1", IPProtocol: "tcp",
		SampledAt: time.Now().AddDate(0, 0, -(defaultRetentionDays + 1)),
	}
	recentSample := model.EtherTrafficSample{
		Interface: "ether1", SrcAddress: "10.0.0.2", IPProtocol: "tcp",
		SampledAt: time.Now(),
	}
	if err := db.Create(&oldSample).Error; err != nil {
		t.Fatalf("failed to create old sample: %v", err)
	}
	if err := db.Create(&recentSample).Error; err != nil {
		t.Fatalf("failed to create recent sample: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	svc.RunScheduledCleanup()

	var remainingLogs []model.IPConnectionLog
	if err := db.Find(&remainingLogs).Error; err != nil {
		t.Fatalf("failed to list remaining logs: %v", err)
	}
	if len(remainingLogs) != 1 || remainingLogs[0].Identity != "recent-log" {
		t.Fatalf("expected only 'recent-log' to survive, got %+v", remainingLogs)
	}

	var remainingSamples []model.EtherTrafficSample
	if err := db.Find(&remainingSamples).Error; err != nil {
		t.Fatalf("failed to list remaining samples: %v", err)
	}
	if len(remainingSamples) != 1 || remainingSamples[0].SrcAddress != "10.0.0.2" {
		t.Fatalf("expected only the recent sample to survive, got %+v", remainingSamples)
	}
}

// TestDeleteEtherTrafficOlderThan_IsHardDelete mirrors
// TestDeleteConnectionLogOlderThan_IsHardDelete for EtherTrafficSample,
// which has the identical embeds-model.Model soft-delete bug.
func TestDeleteEtherTrafficOlderThan_IsHardDelete(t *testing.T) {
	db := openSecurityRetentionTestDB(t)

	old := model.EtherTrafficSample{
		Interface: "ether1", SrcAddress: "10.0.0.1", IPProtocol: "tcp",
		SampledAt: time.Now().AddDate(0, 0, -10),
	}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old sample: %v", err)
	}

	svc := NewSecurityRetentionService(db)
	deleted, err := svc.DeleteEtherTrafficOlderThan(time.Now().AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 row deleted, got %d", deleted)
	}

	var unscopedCount int64
	if err := db.Unscoped().Model(&model.EtherTrafficSample{}).Where("id = ?", old.ID).Count(&unscopedCount).Error; err != nil {
		t.Fatalf("failed to count unscoped: %v", err)
	}
	if unscopedCount != 0 {
		t.Fatalf("expected the old sample to be genuinely (hard) deleted, but it's still present when queried Unscoped()")
	}
}
