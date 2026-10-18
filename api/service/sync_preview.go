package service

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// GetSyncInterfacesAllServers runs GetSyncInterfaces against every
// registered server and concatenates the results -- the sync preview page
// has no per-server filter yet, so it shows every server's live interfaces
// in one list. Each row carries its own ServerID/ServerName (set here, since
// SyncService itself only knows its numeric serverID, not the server's
// display name) so the UI can label which server a row belongs to and so a
// later "sync selected" call can be pinned back to the right one.
func GetSyncInterfacesAllServers(db *gorm.DB, factory SyncServiceFactory) ([]schema.SyncInterfacePreviewResponse, error) {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		return nil, err
	}
	result := make([]schema.SyncInterfacePreviewResponse, 0)
	for _, server := range servers {
		ifaces, err := factory(server.ID).GetSyncInterfaces()
		if err != nil {
			zap.L().Named("SyncService").Error("failed to fetch sync interfaces for server", zap.Uint("server_id", server.ID), zap.Error(err))
			continue
		}
		for i := range ifaces {
			ifaces[i].ServerID = server.ID
			ifaces[i].ServerName = server.Name
		}
		result = append(result, ifaces...)
	}
	return result, nil
}

// GetSyncPeersAllServers mirrors GetSyncInterfacesAllServers for peers.
func GetSyncPeersAllServers(db *gorm.DB, factory SyncServiceFactory, interfaceName string) ([]schema.SyncPeerPreviewResponse, error) {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		return nil, err
	}
	result := make([]schema.SyncPeerPreviewResponse, 0)
	for _, server := range servers {
		peers, err := factory(server.ID).GetSyncPeers(interfaceName)
		if err != nil {
			zap.L().Named("SyncService").Error("failed to fetch sync peers for server", zap.Uint("server_id", server.ID), zap.Error(err))
			continue
		}
		for i := range peers {
			peers[i].ServerID = server.ID
			peers[i].ServerName = server.Name
		}
		result = append(result, peers...)
	}
	return result, nil
}

func (s *SyncService) GetSyncInterfaces() ([]schema.SyncInterfacePreviewResponse, error) {
	ifaces, err := s.fetchMikrotikInterfaces()
	if err != nil {
		return nil, err
	}

	return mapSyncInterfaces(ifaces), nil
}

func (s *SyncService) GetSyncPeers(interfaceName string) ([]schema.SyncPeerPreviewResponse, error) {
	peers, err := s.fetchMikrotikPeers()
	if err != nil {
		return nil, err
	}

	if interfaceName != "" {
		filtered := make([]mikrotik.WireGuardPeer, 0, len(peers))
		for _, peer := range peers {
			if peer.Interface == interfaceName {
				filtered = append(filtered, peer)
			}
		}
		peers = filtered
	}

	return mapSyncPeers(peers), nil
}

// SyncSelectedInterfacesForServer runs the selection against exactly the
// server it came from -- the preview list's rows each carry the ServerID
// they were fetched from (see GetSyncInterfacesAllServers), and a RouterOS
// ".id" like "*3" is only unique within one router, so applying a selection
// to every server (as this used to do) risked a no-op-by-luck rather than a
// guaranteed-correct match. Pinning to one server makes the match exact
// instead of relying on other servers' IDs happening not to collide.
func SyncSelectedInterfacesForServer(factory SyncServiceFactory, serverID uint, interfaceIDs []string) error {
	return factory(serverID).SyncSelectedInterfaces(interfaceIDs)
}

// SyncSelectedPeersForServer mirrors SyncSelectedInterfacesForServer for
// peers.
func SyncSelectedPeersForServer(factory SyncServiceFactory, serverID uint, peerIDs []string) error {
	return factory(serverID).SyncSelectedPeers(peerIDs)
}

func (s *SyncService) SyncSelectedInterfaces(interfaceIDs []string) error {
	if len(interfaceIDs) == 0 {
		return errors.New("no interfaces selected")
	}

	mikrotikIfaces, err := s.fetchMikrotikInterfaces()
	if err != nil {
		return err
	}

	selectedIfaces := filterMikrotikInterfacesByID(mikrotikIfaces, interfaceIDs)
	if len(selectedIfaces) == 0 {
		s.logger.Info("no matching interfaces selected for sync")
		return nil
	}

	dbIfaces, err := s.fetchDBInterfaces()
	if err != nil {
		return err
	}

	return s.syncNewAndUpdatedInterfaces(s.mapMikrotikInterfaces(selectedIfaces), s.mapDBInterfaces(dbIfaces))
}

func (s *SyncService) SyncSelectedPeers(peerIDs []string) error {
	if len(peerIDs) == 0 {
		return errors.New("no peers selected")
	}

	mikrotikPeers, err := s.fetchMikrotikPeers()
	if err != nil {
		return err
	}

	selectedPeers := filterMikrotikPeersByID(mikrotikPeers, peerIDs)
	if len(selectedPeers) == 0 {
		s.logger.Info("no matching peers selected for sync")
		return nil
	}

	dbPeers, err := s.fetchDBPeers()
	if err != nil {
		return err
	}

	return s.syncNewAndUpdatedPeers(s.mapMikrotikPeers(selectedPeers), s.mapDBPeers(dbPeers))
}

func parseOptionalBool(value *string) bool {
	if value == nil {
		return false
	}

	return parseBool(*value)
}

func mapSyncInterfaces(ifaces []mikrotik.WireGuardInterface) []schema.SyncInterfacePreviewResponse {
	result := make([]schema.SyncInterfacePreviewResponse, 0, len(ifaces))
	for _, iface := range ifaces {
		result = append(result, schema.SyncInterfacePreviewResponse{
			ID:         iface.ID,
			Disabled:   parseBool(iface.Disabled),
			Comment:    iface.Comment,
			Name:       iface.Name,
			ListenPort: iface.ListenPort,
			MTU:        iface.MTU,
			IsRunning:  parseOptionalBool(iface.Running),
		})
	}

	return result
}

func mapSyncPeers(peers []mikrotik.WireGuardPeer) []schema.SyncPeerPreviewResponse {
	result := make([]schema.SyncPeerPreviewResponse, 0, len(peers))
	for _, peer := range peers {
		result = append(result, schema.SyncPeerPreviewResponse{
			ID:             peer.ID,
			Disabled:       parseBool(peer.Disabled),
			Comment:        peer.Comment,
			Name:           peer.Name,
			Interface:      peer.Interface,
			AllowedAddress: peer.AllowedAddress,
		})
	}

	return result
}

func filterMikrotikInterfacesByID(ifaces []mikrotik.WireGuardInterface, ids []string) []mikrotik.WireGuardInterface {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}

	result := make([]mikrotik.WireGuardInterface, 0, len(ids))
	for _, iface := range ifaces {
		if _, ok := selected[iface.ID]; ok {
			result = append(result, iface)
		}
	}

	return result
}

func filterMikrotikPeersByID(peers []mikrotik.WireGuardPeer, ids []string) []mikrotik.WireGuardPeer {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}

	result := make([]mikrotik.WireGuardPeer, 0, len(ids))
	for _, peer := range peers {
		if _, ok := selected[peer.ID]; ok {
			result = append(result, peer)
		}
	}

	return result
}
