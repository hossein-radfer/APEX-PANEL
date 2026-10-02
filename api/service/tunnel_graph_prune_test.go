package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestTunnelGraphService(t *testing.T) (*TunnelGraphService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:tunnel_graph_prune_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.DiscoverySnapshot{},
		&model.GraphNode{},
		&model.GraphEdge{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return NewTunnelGraphService(db, nil), db
}

// TestPruneOldSnapshots_HardDeletesStaleRows is the regression test for a
// confirmed, reported production bug: pruneOldSnapshots previously called
// plain .Delete() on DiscoverySnapshot/GraphNode/GraphEdge, all of which
// embed model.Model (carrying gorm.DeletedAt) -- so every "pruned" row was
// only soft-deleted, permanently bloating the database file (5.69M of
// 5.7M graph_nodes rows were found soft-deleted-but-present on a real
// production database). This test creates more than snapshotRetentionCount
// snapshots (each with a node and an edge), prunes, and confirms the stale
// rows are gone even from an Unscoped() query -- a soft-delete-only bug
// would still show them there.
func TestPruneOldSnapshots_HardDeletesStaleRows(t *testing.T) {
	svc, db := newTestTunnelGraphService(t)

	const serverID = uint(1)
	totalSnapshots := snapshotRetentionCount + 5

	var allSnapshotIDs []uint
	for i := 0; i < totalSnapshots; i++ {
		snapshot := model.DiscoverySnapshot{
			ServerID: serverID,
			TakenAt:  time.Now().Add(time.Duration(i) * time.Minute),
			RawJSON:  "{}",
		}
		if err := db.Create(&snapshot).Error; err != nil {
			t.Fatalf("failed to create snapshot %d: %v", i, err)
		}
		allSnapshotIDs = append(allSnapshotIDs, snapshot.ID)

		node := model.GraphNode{SnapshotID: snapshot.ID, Type: "address", MikrotikRefID: fmt.Sprintf("ref-%d", i)}
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("failed to create node %d: %v", i, err)
		}
		edge := model.GraphEdge{SnapshotID: snapshot.ID, FromNodeID: node.ID, ToNodeID: node.ID, Relation: "self"}
		if err := db.Create(&edge).Error; err != nil {
			t.Fatalf("failed to create edge %d: %v", i, err)
		}
	}

	svc.pruneOldSnapshots(serverID)

	// The oldest (totalSnapshots - snapshotRetentionCount) snapshots should
	// now be gone -- genuinely gone, not just soft-deleted.
	staleCount := totalSnapshots - snapshotRetentionCount
	staleSnapshotIDs := allSnapshotIDs[:staleCount]
	keptSnapshotIDs := allSnapshotIDs[staleCount:]

	var remainingStaleSnapshots int64
	if err := db.Unscoped().Model(&model.DiscoverySnapshot{}).Where("id IN ?", staleSnapshotIDs).Count(&remainingStaleSnapshots).Error; err != nil {
		t.Fatalf("failed to count stale snapshots: %v", err)
	}
	if remainingStaleSnapshots != 0 {
		t.Fatalf("expected all %d stale snapshots to be hard-deleted, found %d still present (even Unscoped)", staleCount, remainingStaleSnapshots)
	}

	var remainingStaleNodes int64
	if err := db.Unscoped().Model(&model.GraphNode{}).Where("snapshot_id IN ?", staleSnapshotIDs).Count(&remainingStaleNodes).Error; err != nil {
		t.Fatalf("failed to count stale nodes: %v", err)
	}
	if remainingStaleNodes != 0 {
		t.Fatalf("expected all stale nodes to be hard-deleted, found %d still present (even Unscoped) -- this is the exact soft-delete bug being regression-tested", remainingStaleNodes)
	}

	var remainingStaleEdges int64
	if err := db.Unscoped().Model(&model.GraphEdge{}).Where("snapshot_id IN ?", staleSnapshotIDs).Count(&remainingStaleEdges).Error; err != nil {
		t.Fatalf("failed to count stale edges: %v", err)
	}
	if remainingStaleEdges != 0 {
		t.Fatalf("expected all stale edges to be hard-deleted, found %d still present (even Unscoped)", remainingStaleEdges)
	}

	// The most recent snapshotRetentionCount snapshots must survive untouched.
	var keptCount int64
	if err := db.Model(&model.DiscoverySnapshot{}).Where("id IN ?", keptSnapshotIDs).Count(&keptCount).Error; err != nil {
		t.Fatalf("failed to count kept snapshots: %v", err)
	}
	if int(keptCount) != len(keptSnapshotIDs) {
		t.Fatalf("expected all %d retained snapshots to survive, found %d", len(keptSnapshotIDs), keptCount)
	}
}

// TestPruneOldSnapshots_NoOpWhenUnderRetentionLimit confirms pruning
// doesn't touch anything when there aren't more snapshots than
// snapshotRetentionCount yet -- the early-return path.
func TestPruneOldSnapshots_NoOpWhenUnderRetentionLimit(t *testing.T) {
	svc, db := newTestTunnelGraphService(t)

	const serverID = uint(1)
	for i := 0; i < 3; i++ {
		snapshot := model.DiscoverySnapshot{ServerID: serverID, TakenAt: time.Now(), RawJSON: "{}"}
		if err := db.Create(&snapshot).Error; err != nil {
			t.Fatalf("failed to create snapshot: %v", err)
		}
	}

	svc.pruneOldSnapshots(serverID)

	var count int64
	if err := db.Model(&model.DiscoverySnapshot{}).Where("server_id = ?", serverID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count snapshots: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected all 3 snapshots to survive when under the retention limit, got %d", count)
	}
}
