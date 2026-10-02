package service

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func TestEnsurePeerAccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	service := NewWGPeer(db, nil, nil, nil, nil, nil, nil)

	ownerResellerID := uint(11)
	otherResellerID := uint(12)
	peer := model.Peer{
		UUID:           "peer-scope-uuid",
		PeerID:         "peer-scope-id",
		Name:           "peer-scope",
		PrivateKey:     "private-key",
		PublicKey:      "public-key",
		Interface:      "wg0",
		AllowedAddress: "10.10.10.2/32",
		Endpoint:       "example.com",
		EndpointPort:   "51820",
		ResellerID:     &ownerResellerID,
	}

	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	if err := service.EnsurePeerAccess(peer.ID, nil); err != nil {
		t.Fatalf("expected admin scope to access peer, got %v", err)
	}

	if err := service.EnsurePeerAccess(peer.ID, &ownerResellerID); err != nil {
		t.Fatalf("expected owner reseller to access peer, got %v", err)
	}

	err = service.EnsurePeerAccess(peer.ID, &otherResellerID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for non-owner reseller, got %v", err)
	}
}
