package traffic

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.Peer{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestApplyResellerQuota_DisablesResellerAndPeers(t *testing.T) {
	db := openTestDB(t)

	// create reseller with small quota
	res := model.Reseller{
		Name:       "test-reseller",
		QuotaBytes: func() *int64 { v := int64(100); return &v }(),
		UsedBytes:  0,
		IsActive:   true,
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// create peer belonging to reseller
	peer := model.Peer{
		UUID:           "uuid-1",
		PeerID:         "peer-1",
		Name:           "peer1",
		PrivateKey:     "priv",
		PublicKey:      "pub",
		Interface:      "wg0",
		AllowedAddress: "10.0.0.2/32",
		ResellerID:     &res.ID,
		Disabled:       false,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	// create calculator with nil mikrotik adaptor so it won't attempt external calls
	calc := &Calculator{db: db, mikrotikAdaptor: nil}

	// apply quota with delta greater than quota
	calc.applyResellerQuota(&res.ID, "wg0", 200)

	// re-fetch reseller and peer
	var updatedRes model.Reseller
	if err := db.First(&updatedRes, "id = ?", res.ID).Error; err != nil {
		t.Fatalf("failed to fetch reseller: %v", err)
	}
	if updatedRes.IsActive {
		t.Fatalf("expected reseller to be disabled")
	}

	var updatedPeer model.Peer
	if err := db.First(&updatedPeer, "id = ?", peer.ID).Error; err != nil {
		t.Fatalf("failed to fetch peer: %v", err)
	}
	if !updatedPeer.Disabled {
		t.Fatalf("expected peer to be disabled")
	}
}
