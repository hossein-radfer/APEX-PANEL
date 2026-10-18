package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils/wireguard"
)

// SyncService reconciles this panel's DB (Peer/Interface rows) against a
// SINGLE Mikrotik router's live state -- never across every registered
// server at once. serverID is required at construction (see
// NewSyncServiceForServer) and every DB read/write this service performs is
// explicitly scoped to it (.Where("server_id = ?", s.serverID)), and every
// delete goes through deleteStaleRow, the ONE place in this file allowed to
// remove a Peer/Interface row.
//
// This shape exists to close a confirmed danger: comparing a partial source
// of truth (one router's live peer/interface list) against an unscoped
// destination set (every Peer/Interface row in the whole database) and
// deleting whatever doesn't match is only safe when there is exactly one
// server in the whole system. The moment a second server exists, the SAME
// comparison would see server B's peers as simply absent from server A's
// live list and permanently delete them -- not a hypothetical, this is
// exactly the shape of bug that already happened once in this file (see
// removeStaleInterfaces' own historical `s.db.Delete(dbIface.ID).Unscoped()`
// mis-call, fixed by routing through deleteStaleRow below). Any FUTURE
// "compare remote state to a DB table and delete what's missing" job in
// this codebase should copy this shape (required serverID at construction,
// a single private delete chokepoint that always re-applies the scope)
// rather than reintroduce an unscoped comparison.
//
// SyncPeersAllServers/SyncInterfacesAllServers below are the only
// multi-server-aware entrypoints -- they construct one SyncService per
// registered server and run each one's single-server logic independently,
// so a failure or a stale-row cleanup on one server can never affect
// another's rows.
type SyncService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	configService   *ConfigGenerator
	qrCodeService   *QRCodeGenerator
	serverID        uint
	logger          *zap.Logger
}

// NewSyncServiceForServer builds a SyncService scoped to exactly one
// server. serverID is required (not optional/nilable) -- see this type's
// own doc comment for why a sync/stale-row-deletion service must always
// know exactly which server's rows it's allowed to touch.
func NewSyncServiceForServer(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, configService *ConfigGenerator, qrCodeService *QRCodeGenerator, serverID uint) *SyncService {
	return &SyncService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		configService:   configService,
		qrCodeService:   qrCodeService,
		serverID:        serverID,
		logger:          zap.L().Named("SyncService").With(zap.Uint("server_id", serverID)),
	}
}

// SyncServiceFactory builds a per-server SyncService on demand --
// SyncPeersAllServers/SyncInterfacesAllServers below take this rather than
// a *SyncService directly, since a single instance is permanently bound to
// one serverID and a multi-server sync needs a fresh one per server.
type SyncServiceFactory func(serverID uint) *SyncService

// NewSyncServiceFactory returns a SyncServiceFactory closed over the
// shared dependencies every per-server SyncService needs.
func NewSyncServiceFactory(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, configService *ConfigGenerator, qrCodeService *QRCodeGenerator) SyncServiceFactory {
	return func(serverID uint) *SyncService {
		return NewSyncServiceForServer(db, mikrotikAdaptor, configService, qrCodeService, serverID)
	}
}

// SyncPeersAllServers runs SyncPeers independently against every server
// row in the database -- the admin-facing "sync peers" action operates
// panel-wide, not against one server at a time (there is no per-server
// picker at this call site yet). A failure on one server is logged and
// does not stop the others from being attempted, mirroring every other
// multi-resource job in this codebase's own "one dead source never blocks
// the rest" convention; the first error encountered (if any) is still
// returned so the HTTP layer can report that SOMETHING failed.
func SyncPeersAllServers(db *gorm.DB, factory SyncServiceFactory) error {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		return err
	}
	var firstErr error
	for _, server := range servers {
		if err := factory(server.ID).SyncPeers(); err != nil {
			zap.L().Named("SyncService").Error("failed to sync peers for server", zap.Uint("server_id", server.ID), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// SyncInterfacesAllServers mirrors SyncPeersAllServers for SyncInterfaces.
func SyncInterfacesAllServers(db *gorm.DB, factory SyncServiceFactory) error {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		return err
	}
	var firstErr error
	for _, server := range servers {
		if err := factory(server.ID).SyncInterfaces(); err != nil {
			zap.L().Named("SyncService").Error("failed to sync interfaces for server", zap.Uint("server_id", server.ID), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *SyncService) SyncPeers() error {
	mikrotikPeers, err := s.fetchMikrotikPeers()
	if err != nil {
		return err
	}

	dbPeers, err := s.fetchDBPeers()
	if err != nil {
		return err
	}

	if len(mikrotikPeers) == len(dbPeers) {
		s.logger.Info("no changes detected in peers, skipping sync")
		return nil
	}

	mikrotikMap := s.mapMikrotikPeers(mikrotikPeers)
	dbMap := s.mapDBPeers(dbPeers)

	if err := s.syncNewAndUpdatedPeers(mikrotikMap, dbMap); err != nil {
		return err
	}

	if err := s.removeStalePeers(mikrotikMap, dbMap); err != nil {
		return err
	}

	s.logger.Info("successfully synced peers with mikrotik")
	return nil
}

func (s *SyncService) SyncInterfaces() error {
	mikrotikIfaces, err := s.fetchMikrotikInterfaces()
	if err != nil {
		return err
	}

	dbIfaces, err := s.fetchDBInterfaces()
	if err != nil {
		return err
	}

	if len(mikrotikIfaces) == len(dbIfaces) {
		s.logger.Info("no changes detected in interfaces, skipping sync")
		return nil
	}

	mikrotikMap := s.mapMikrotikInterfaces(mikrotikIfaces)
	dbMap := s.mapDBInterfaces(dbIfaces)

	if err := s.syncNewAndUpdatedInterfaces(mikrotikMap, dbMap); err != nil {
		return err
	}

	if err := s.removeStaleInterfaces(mikrotikMap, dbMap); err != nil {
		return err
	}

	s.logger.Info("successfully synced interfaces with mikrotik")
	return nil
}

func (s *SyncService) fetchMikrotikPeers() ([]mikrotik.WireGuardPeer, error) {
	peers, err := s.mikrotikAdaptor.ForServer(s.serverID).FetchWgPeers(context.Background())
	if err != nil {
		s.logger.Error("failed to get peers from Mikrotik", zap.Error(err))
	}
	return peers, err
}

// fetchDBPeers returns ONLY this SyncService's own server's Peer rows --
// see this type's own doc comment for why every read/write here is scoped
// this way.
func (s *SyncService) fetchDBPeers() ([]model.Peer, error) {
	var peers []model.Peer
	err := s.db.Where("server_id = ?", s.serverID).Find(&peers).Error
	if err != nil {
		s.logger.Error("failed to get peers from database", zap.Error(err))
	}
	return peers, err
}

func (s *SyncService) mapMikrotikPeers(peers []mikrotik.WireGuardPeer) map[string]mikrotik.WireGuardPeer {
	m := make(map[string]mikrotik.WireGuardPeer)
	for _, p := range peers {
		m[p.ID] = p
	}
	return m
}

func (s *SyncService) mapDBPeers(peers []model.Peer) map[string]model.Peer {
	m := make(map[string]model.Peer)
	for _, p := range peers {
		m[p.PeerID] = p
	}
	return m
}

func (s *SyncService) syncNewAndUpdatedPeers(peers map[string]mikrotik.WireGuardPeer, dbMap map[string]model.Peer) error {
	for id, peer := range peers {
		if peer.PrivateKey == nil {
			s.logger.Error("missing private key", zap.String("peer", id))
			continue
		}

		server, err := s.fetchServer()
		if err != nil {
			return err
		}

		dbIface, err := s.fetchInterface(peer.Interface, id)
		if err != nil {
			return err
		}

		dbPeer, exists := dbMap[id]
		if !exists {
			dbPeer = model.Peer{
				UUID:                uuid.New().String(),
				ServerID:            s.serverID,
				PeerID:              peer.ID,
				PersistentKeepalive: common.DefaultKeepalive,
			}
		}

		dbPeer.Disabled = parseBool(peer.Disabled)
		dbPeer.Comment = peer.Comment
		dbPeer.Name = peer.Name
		dbPeer.PrivateKey = *peer.PrivateKey
		dbPeer.PublicKey = peer.PublicKey
		dbPeer.Interface = dbIface.Name
		dbPeer.AllowedAddress = peer.AllowedAddress
		dbPeer.Endpoint = server.IPAddress
		dbPeer.EndpointPort = dbIface.ListenPort

		config := s.buildConfig(peer, dbPeer, dbIface)

		if err := s.configService.BuildPeerConfig(config, dbPeer.UUID); err != nil {
			s.logger.Error("failed to build peer config", zap.String("id", id), zap.Error(err))
			return err
		}

		if err := s.qrCodeService.BuildPeerQRCode(config, dbPeer.UUID); err != nil {
			s.logger.Error("failed to build QR code", zap.String("id", id), zap.Error(err))
			return err
		}

		if err := s.db.Save(&dbPeer).Error; err != nil {
			s.logger.Error("failed to upsert peer", zap.String("id", id), zap.Error(err))
			return err
		}
	}
	return nil
}

// removeStalePeers deletes every DB peer row (belonging to THIS SyncService's
// own server -- dbMap, built from fetchDBPeers, only ever contains rows
// already scoped that way) that mikrotikMap (this same server's live
// peer list) no longer reports. Goes through deleteStaleRow, which
// re-applies the server_id scope at delete time as a second, independent
// guard -- see this type's own doc comment.
func (s *SyncService) removeStalePeers(mikrotikMap map[string]mikrotik.WireGuardPeer, dbMap map[string]model.Peer) error {
	for id, peer := range dbMap {
		if _, found := mikrotikMap[id]; !found {
			if err := s.deleteStaleRow(&model.Peer{}, peer.ID); err != nil {
				s.logger.Error("failed to delete peer", zap.String("peerId", id), zap.Error(err))
				return err
			}
			s.logger.Info("deleted stale peer from DB", zap.String("peerId", id))
		}
	}
	return nil
}

// deleteStaleRow is the ONLY place in this file permitted to delete a
// Peer/Interface row -- it re-applies s.serverID as a WHERE clause on the
// delete itself (not just on however the caller found the row), so even a
// future bug in how a caller BUILT its candidate-for-deletion set can never
// result in a row from a DIFFERENT server being removed. See this type's
// own doc comment for the exact incident class this closes.
func (s *SyncService) deleteStaleRow(model_ any, id uint) error {
	return s.db.Unscoped().Where("server_id = ?", s.serverID).Delete(model_, id).Error
}

func (s *SyncService) fetchServer() (model.Server, error) {
	var server model.Server
	err := s.db.First(&server, s.serverID).Error
	if err != nil {
		s.logger.Error("failed to get server", zap.Error(err))
	}
	return server, err
}

func (s *SyncService) fetchInterface(name, peerID string) (model.Interface, error) {
	var iface model.Interface
	err := s.db.Where("server_id = ? AND name = ?", s.serverID, name).First(&iface).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			s.logger.Error("interface not found", zap.String("peerId", peerID), zap.String("interfaceId", name))
			return iface, fmt.Errorf("interface %s not found for peer %s", name, peerID)
		}
		s.logger.Error("failed to get interface", zap.String("peerId", peerID), zap.Error(err))
	}
	return iface, err
}

func (s *SyncService) buildDBPeer(peer mikrotik.WireGuardPeer, server model.Server, iface model.Interface) model.Peer {
	return model.Peer{
		UUID:                uuid.New().String(),
		ServerID:            s.serverID,
		PeerID:              peer.ID,
		Disabled:            parseBool(peer.Disabled),
		Comment:             peer.Comment,
		Name:                peer.Name,
		PrivateKey:          *peer.PrivateKey,
		PublicKey:           peer.PublicKey,
		Interface:           peer.Interface,
		AllowedAddress:      peer.AllowedAddress,
		Endpoint:            server.IPAddress,
		EndpointPort:        iface.ListenPort,
		PersistentKeepalive: common.DefaultKeepalive,
	}
}

func (s *SyncService) buildConfig(peer mikrotik.WireGuardPeer, dbPeer model.Peer, iface model.Interface) string {
	return fmt.Sprintf(wireguard.Template,
		*peer.PrivateKey,
		dbPeer.AllowedAddress,
		common.DefaultDns,
		iface.PublicKey,
		dbPeer.Endpoint,
		dbPeer.EndpointPort,
		common.AllowedIpsIncludeLocal,
		dbPeer.PersistentKeepalive,
	)
}

func (s *SyncService) fetchMikrotikInterfaces() ([]mikrotik.WireGuardInterface, error) {
	ifaces, err := s.mikrotikAdaptor.ForServer(s.serverID).FetchWgInterfaces(context.Background())
	if err != nil {
		s.logger.Error("failed to get interfaces from Mikrotik", zap.Error(err))
	}
	return ifaces, err
}

// fetchDBInterfaces returns ONLY this SyncService's own server's Interface
// rows -- see this type's own doc comment.
func (s *SyncService) fetchDBInterfaces() ([]model.Interface, error) {
	var ifaces []model.Interface
	err := s.db.Where("server_id = ?", s.serverID).Find(&ifaces).Error
	if err != nil {
		s.logger.Error("failed to get interfaces from database", zap.Error(err))
	}
	return ifaces, err
}

func (s *SyncService) mapMikrotikInterfaces(ifaces []mikrotik.WireGuardInterface) map[string]mikrotik.WireGuardInterface {
	m := make(map[string]mikrotik.WireGuardInterface)
	for _, iface := range ifaces {
		m[iface.ID] = iface
	}
	return m
}

func (s *SyncService) mapDBInterfaces(ifaces []model.Interface) map[string]model.Interface {
	m := make(map[string]model.Interface)
	for _, iface := range ifaces {
		m[iface.InterfaceID] = iface
	}
	return m
}

func (s *SyncService) syncNewAndUpdatedInterfaces(ifaceMap map[string]mikrotik.WireGuardInterface, dbMap map[string]model.Interface) error {
	for id, mikrotikIface := range ifaceMap {
		dbIface, exists := dbMap[id]
		if !exists {
			dbIface = model.Interface{
				ServerID:    s.serverID,
				InterfaceID: mikrotikIface.ID,
			}
		}

		dbIface.Disabled = parseBool(mikrotikIface.Disabled)
		dbIface.Comment = mikrotikIface.Comment
		dbIface.Name = mikrotikIface.Name
		dbIface.PrivateKey = mikrotikIface.PrivateKey
		dbIface.PublicKey = mikrotikIface.PublicKey
		dbIface.ListenPort = mikrotikIface.ListenPort

		if err := s.db.Save(&dbIface).Error; err != nil {
			s.logger.Error("failed to upsert interface", zap.String("id", id), zap.Error(err))
			return err
		}
	}
	return nil
}

// removeStaleInterfaces deletes every DB interface row (belonging to THIS
// SyncService's own server) that mikrotikMap no longer reports. This is
// also where a separate, confirmed delete-call bug lived
// (s.db.Delete(dbIface.ID).Unscoped() -- Delete on an *bare ID* with no
// model/table context is not a valid GORM delete-by-primary-key call at
// all) -- deleteStaleRow fixes both issues at once: the wrong-call-shape
// bug, and (now) the missing server scope.
func (s *SyncService) removeStaleInterfaces(mikrotikMap map[string]mikrotik.WireGuardInterface, dbMap map[string]model.Interface) error {
	for id, dbIface := range dbMap {
		if _, exists := mikrotikMap[id]; !exists {
			if err := s.deleteStaleRow(&model.Interface{}, dbIface.ID); err != nil {
				s.logger.Error("failed to delete interface", zap.String("interfaceId", id), zap.Error(err))
				return err
			}
			s.logger.Info("deleted stale interface from DB", zap.String("interfaceId", id))
		}
	}
	return nil
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
