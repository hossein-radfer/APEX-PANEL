package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"context"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
	"github.com/maahdima/mwp/api/utils/wireguard"
)

var (
	peerConfigsPath string
)

func init() {
	appCfg := config.GetAppConfig()
	peerConfigsPath = filepath.Join(appCfg.PeerFilesDir, "config")
	if err := os.MkdirAll(peerConfigsPath, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create peer config directory: %v", err))
	}
}

type ConfigGenerator struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewConfigGenerator(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *ConfigGenerator {
	return &ConfigGenerator{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("ConfigGenerator"),
	}
}

// resolveDnsServers reads the router's own configured DNS servers
// (RouterOS /ip/dns, same source device_data.go's own getDNSConfig uses)
// so a peer's config ships with whatever DNS the admin has actually set on
// the router -- a confirmed, reported bug: this used to always hand out a
// hardcoded common.DefaultDns value completely independent of the router's
// real /ip/dns settings, so changing DNS on the router had no effect on
// what clients received. Falls back to common.DefaultDns only if the
// router read fails, so a transient RouterOS API hiccup can't block peer
// creation/regeneration entirely.
func resolveDnsServers(mikrotikAdaptor *mikrotik.Adaptor, logger *zap.Logger) string {
	dnsConfig, err := mikrotikAdaptor.FetchDNSConfig(context.Background())
	if err != nil || dnsConfig.Servers == "" {
		logger.Warn("failed to fetch router DNS config, falling back to default", zap.Error(err))
		return common.DefaultDns
	}
	return dnsConfig.Servers
}

func (c *ConfigGenerator) GetPeerConfig(id uint) (configPath string, err error) {
	var peer model.Peer

	if err = c.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.logger.Error("peer not found in database", zap.Uint("id", id))
			return
		}
		c.logger.Error("failed to get peer from database", zap.Uint("id", id), zap.Error(err))
		return
	}

	configPath = fmt.Sprintf("%s/%s.conf", peerConfigsPath, peer.UUID)

	if _, statErr := os.Stat(configPath); statErr != nil {
		// The DB row survived (e.g. a database-only backup/restore, or a
		// server migration where PeerFilesDir wasn't copied across), but
		// the actual .conf file never made the trip -- every field needed
		// to rebuild it verbatim is still in the DB, so regenerate it here
		// instead of 404ing forever. See RegeneratePeerAssets for the
		// single source of truth this and BuildPeerQRCode's equivalent
		// on-miss path both call.
		c.logger.Warn("peer config file missing on disk, regenerating from database", zap.Uint("id", id), zap.String("uuid", peer.UUID))
		if regenErr := c.regeneratePeerConfigFile(peer); regenErr != nil {
			c.logger.Error("failed to regenerate missing peer config", zap.Uint("id", id), zap.Error(regenErr))
			return "", regenErr
		}
	}

	return configPath, nil
}

// regeneratePeerConfigFile rebuilds and writes a peer's .conf file purely
// from data already in the database -- the interface's public key is the
// one piece of data not on the Peer row itself, looked up by name (see
// model.Peer.Interface, which stores the WireGuard interface's name, not
// a numeric foreign key).
func (c *ConfigGenerator) regeneratePeerConfigFile(peer model.Peer) error {
	var iface model.Interface
	if err := c.db.Where("name = ?", peer.Interface).First(&iface).Error; err != nil {
		return fmt.Errorf("failed to look up interface %q for peer regeneration: %w", peer.Interface, err)
	}

	configText := buildPeerConfigString(peer, iface.PublicKey, c.mikrotikAdaptor, c.logger)
	return c.BuildPeerConfig(configText, peer.UUID)
}

// buildPeerConfigString renders a WireGuard client config from an
// existing Peer row -- used when regenerating a missing .conf/.jpeg for
// an existing peer (see regeneratePeerConfigFile / QRCodeGenerator's
// regeneratePeerQRCodeFile). Deliberately NOT reused by
// WgPeer.generatePeerAssets (peer.go) at creation time: that path takes
// the freshly-generated private key as an explicit parameter rather than
// reading it back off the not-yet-necessarily-identical dbPeer.PrivateKey
// (sourced from RouterOS's own echo of the peer, mtPeer.PrivateKey) --
// collapsing the two into one shared function would silently change which
// value creation uses if the two ever diverge, which is not this fix's
// concern.
func buildPeerConfigString(peer model.Peer, ifacePublicKey string, mikrotikAdaptor *mikrotik.Adaptor, logger *zap.Logger) string {
	dnsServers := resolveDnsServers(mikrotikAdaptor, logger)
	if peer.DNSServers != nil && *peer.DNSServers != "" {
		dnsServers = *peer.DNSServers
	}

	return fmt.Sprintf(
		wireguard.Template,
		peer.PrivateKey,
		peer.AllowedAddress,
		dnsServers,
		ifacePublicKey,
		peer.Endpoint,
		peer.EndpointPort,
		common.AllowedIpsIncludeLocal,
		peer.PersistentKeepalive,
	)
}

func (c *ConfigGenerator) GetUserConfig(uuid string) (configPath string, err error) {
	var peer model.Peer

	if err = c.db.First(&peer, "uuid = ?", uuid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.logger.Error("peer not found in database", zap.String("uuid", uuid))
			return
		}
		c.logger.Error("failed to get peer from database", zap.String("uuid", uuid), zap.Error(err))
		return
	}

	utils.IsPeerSharable(peer.IsShared, peer.ShareExpireTime)
	if !peer.IsShared {
		return "", common.ErrPeerNotShared
	}

	configPath = fmt.Sprintf("%s/%s.conf", peerConfigsPath, peer.UUID)

	if _, statErr := os.Stat(configPath); statErr != nil {
		c.logger.Warn("shared peer config file missing on disk, regenerating from database", zap.String("uuid", uuid))
		if regenErr := c.regeneratePeerConfigFile(peer); regenErr != nil {
			c.logger.Error("failed to regenerate missing shared peer config", zap.String("uuid", uuid), zap.Error(regenErr))
			return "", regenErr
		}
	}

	return configPath, nil
}

func (c *ConfigGenerator) BuildPeerConfig(config string, uuid string) error {
	filePath := fmt.Sprintf("%s/%s.conf", peerConfigsPath, uuid)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(config); err != nil {
		return fmt.Errorf("failed to write config to file: %w", err)
	}

	return nil
}

func (c *ConfigGenerator) RemovePeerConfig(id uint) error {
	var peer model.Peer

	if err := c.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.logger.Error("peer not found in database", zap.Uint("id", id))
			return err
		}
		c.logger.Error("failed to get peer from database", zap.Uint("id", id), zap.Error(err))
		return err
	}

	configPath := fmt.Sprintf("%s/%s.conf", peerConfigsPath, peer.UUID)

	err := os.Remove(configPath)
	if err != nil {
		c.logger.Error("failed to remove Config file", zap.String("path", configPath), zap.Error(err))
		return err
	}

	return nil
}
