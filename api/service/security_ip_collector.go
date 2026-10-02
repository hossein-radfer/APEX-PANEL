package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// SecurityIPCollectorService keeps model.IPConnectionLog up to date --
// every scheduled tick (see cmd/main.go's gocron registration) reads the
// CURRENT connected IP for every WireGuard peer and User Manager account
// directly from the Mikrotik router, and reconciles it against whatever
// IPConnectionLog row is currently open (DisconnectedAt IS NULL) for that
// identity. This is the backend for the admin's Security page requirement:
// "این یوزر از اول تا به امروز با این ایپی‌ها وصل شده" (this user has been
// connected from these IPs, from this time to that time). Per-session
// usage is NOT accrued here -- it's computed on demand from the already-
// correct model.UsageSnapshot rows (see SecurityService.GetIdentityHistory)
// within each row's own [ConnectedAt, DisconnectedAt) window, so this
// collector never has to duplicate UsageSnapshotWriter's own delta/reset
// accounting logic.
type SecurityIPCollectorService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	geoIP           *GeoIPService
	logger          *zap.Logger
}

func NewSecurityIPCollectorService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, geoIP *GeoIPService) *SecurityIPCollectorService {
	return &SecurityIPCollectorService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		geoIP:           geoIP,
		logger:          zap.L().Named("SecurityIPCollectorService"),
	}
}

// Collect is the scheduled job entrypoint -- WireGuard and User Manager
// are each independently best-effort (a fetch failure on one protocol
// never blocks the other), mirroring every other job in this codebase's
// own "one dead source never stops the rest" convention.
func (s *SecurityIPCollectorService) Collect() {
	ctx := context.Background()

	if peers, err := s.mikrotikAdaptor.FetchWgPeers(ctx); err != nil {
		s.logger.Warn("failed to fetch wireguard peers for security IP collection", zap.Error(err))
	} else {
		var dbPeers []model.Peer
		s.db.Find(&dbPeers)
		peerByRouterID := make(map[string]model.Peer, len(dbPeers))
		for _, p := range dbPeers {
			peerByRouterID[p.PeerID] = p
		}

		stillOnline := make(map[string]bool, len(peers))
		for _, peer := range peers {
			if peer.CurrentEndpointAddress == nil || *peer.CurrentEndpointAddress == "" {
				continue
			}
			dbPeer, ok := peerByRouterID[peer.ID]
			if !ok {
				continue
			}
			peerID := dbPeer.ID
			stillOnline[dbPeer.Name] = true
			s.reconcile(model.UsageProtocolWireGuard, dbPeer.Name, &peerID, nil, *peer.CurrentEndpointAddress)
		}
		// Close every row this protocol still has open for an identity
		// that did NOT appear in this tick's active list -- confirmed,
		// reported bug: reconcile() above only ever OPENS/keeps-open a
		// row when the peer is actively reporting an endpoint; a peer
		// that simply disconnects (drops out of the router's active
		// list entirely, not just changes IP) was never revisited, so
		// its row's DisconnectedAt stayed nil forever, making
		// ApplicationService.GetOnlineCount see it as permanently
		// "online" even long after the real disconnect.
		s.closeStaleOpenRows(model.UsageProtocolWireGuard, stillOnline)
	}

	if sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(ctx); err != nil {
		s.logger.Warn("failed to fetch ppp active sessions for security IP collection", zap.Error(err))
	} else {
		var accounts []model.UserManagerAccount
		s.db.Find(&accounts)
		accountByUsername := make(map[string]model.UserManagerAccount, len(accounts))
		for _, a := range accounts {
			accountByUsername[a.Username] = a
		}

		stillOnline := make(map[string]bool, len(sessions))
		for _, session := range sessions {
			if session.CallerID == "" || session.Name == "" {
				continue
			}
			account, ok := accountByUsername[session.Name]
			if !ok {
				continue
			}
			accountID := account.ID
			stillOnline[session.Name] = true
			s.reconcile(model.UsageProtocolUserManager, session.Name, nil, &accountID, session.CallerID)
		}
		s.closeStaleOpenRows(model.UsageProtocolUserManager, stillOnline)
	}
}

// closeStaleOpenRows closes (sets DisconnectedAt) every currently-open
// IPConnectionLog row for this protocol whose Identity was NOT seen in
// this tick's active-session list -- see Collect's own doc comment on
// the bug this fixes.
func (s *SecurityIPCollectorService) closeStaleOpenRows(protocol string, stillOnline map[string]bool) {
	var openRows []model.IPConnectionLog
	if err := s.db.Where("protocol = ? AND disconnected_at IS NULL", protocol).Find(&openRows).Error; err != nil {
		s.logger.Error("failed to list open ip connection log rows", zap.String("protocol", protocol), zap.Error(err))
		return
	}
	now := time.Now()
	for _, row := range openRows {
		if stillOnline[row.Identity] {
			continue
		}
		if err := s.db.Model(&model.IPConnectionLog{}).Where("id = ?", row.ID).
			Update("disconnected_at", now).Error; err != nil {
			s.logger.Error("failed to close stale ip connection log row", zap.Uint("id", row.ID), zap.Error(err))
		}
	}
}

// reconcile is the core per-identity logic: if an IPConnectionLog row is
// already open for (protocol, identity) and its IP still matches, nothing
// changes. If the IP has changed (or there was no open row at all), the
// old row (if any) is closed and a new one opened at the new IP -- so a
// roaming client's full IP history is a clean sequence of
// [ConnectedAt, DisconnectedAt) windows, never overlapping.
func (s *SecurityIPCollectorService) reconcile(protocol, identity string, peerID, accountID *uint, ipAddress string) {
	var open model.IPConnectionLog
	err := s.db.Where("protocol = ? AND identity = ? AND disconnected_at IS NULL", protocol, identity).
		Order("id desc").First(&open).Error

	if err == nil {
		if open.IPAddress == ipAddress {
			return
		}
		now := time.Now()
		if closeErr := s.db.Model(&model.IPConnectionLog{}).Where("id = ?", open.ID).
			Update("disconnected_at", now).Error; closeErr != nil {
			s.logger.Error("failed to close previous ip connection log row", zap.Uint("id", open.ID), zap.Error(closeErr))
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to look up open ip connection log row", zap.String("protocol", protocol), zap.String("identity", identity), zap.Error(err))
		return
	}

	geo, geoErr := s.geoIP.Lookup(ipAddress)
	if geoErr != nil {
		s.logger.Warn("geoip lookup failed, recording connection without location", zap.String("ip", ipAddress), zap.Error(geoErr))
		geo = &GeoIPLookupResult{}
	}

	newRow := model.IPConnectionLog{
		Protocol:    protocol,
		Identity:    identity,
		PeerID:      peerID,
		AccountID:   accountID,
		IPAddress:   ipAddress,
		ConnectedAt: time.Now(),
		Country:     geo.Country, Region: geo.Region, City: geo.City,
		Lat: geo.Lat, Lon: geo.Lon, Timezone: geo.Timezone,
		ASN: geo.ASN, ISP: geo.ISP,
	}
	if err := s.db.Create(&newRow).Error; err != nil {
		s.logger.Error("failed to create ip connection log row", zap.String("protocol", protocol), zap.String("identity", identity), zap.Error(err))
	}
}
