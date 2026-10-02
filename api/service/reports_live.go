package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// ReportsLiveService answers the two Reports-section queries that
// genuinely need a live read (currently-online users, panel health) --
// split from ReportsService (which is a pure historical-data reader over
// UsageSnapshot/ResourceSample) since these two need the Mikrotik
// adaptor, mirroring how V2RaySyncService/WgPeer are themselves split
// from the purely-DB-reading services in this codebase.
type ReportsLiveService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewReportsLiveService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *ReportsLiveService {
	return &ReportsLiveService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("ReportsLiveService"),
	}
}

// GetOnlineUsers is report 8: every currently-connected client across all
// three protocols, plus a grand total -- WireGuard is counted only (a
// live per-peer connected/disconnected read is already a relatively
// heavy Mikrotik call; see WgPeer.GetPeersData's own equivalent), User
// Manager and V2Ray are listed by name since both already have a cheap
// source for that (a single FetchPPPActiveSessions call, and V2Ray's own
// sync-job-cached IsOnline column, respectively).
func (s *ReportsLiveService) GetOnlineUsers() (*schema.ReportsOnlineUsersResponse, error) {
	users := make([]schema.ReportsOnlineUser, 0)

	if sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(context.Background()); err != nil {
		s.logger.Warn("failed to fetch ppp active sessions for online users report", zap.Error(err))
	} else {
		// A confirmed, reported gap: unlike the WireGuard and V2Ray branches
		// below (each of which joins back to its own DB table for a
		// user-facing "location"), this branch never looked up the matching
		// UserManagerAccount row at all, so Location was always "" (rendered
		// as "—" by the frontend). Group is the closest UserManager
		// equivalent of "location" this codebase has -- mirrors the exact
		// same "load once, map by username" pattern the WireGuard branch
		// below already uses for Peer.Interface.
		var accounts []model.UserManagerAccount
		s.db.Find(&accounts)
		groupByUsername := make(map[string]string, len(accounts))
		for _, a := range accounts {
			groupByUsername[a.Username] = a.Group
		}
		for _, session := range sessions {
			users = append(users, schema.ReportsOnlineUser{
				Name:     session.Name,
				Protocol: model.UsageProtocolUserManager,
				Location: groupByUsername[session.Name],
			})
		}
	}

	var wgPeers []mikrotik.WireGuardPeer
	wgOnlineCount := 0
	if peers, err := s.mikrotikAdaptor.FetchWgPeers(context.Background()); err != nil {
		s.logger.Warn("failed to fetch wireguard peers for online users report", zap.Error(err))
	} else {
		wgPeers = peers
		var dbPeers []model.Peer
		s.db.Find(&dbPeers)
		type peerInfo struct {
			name        string
			ifaceName   string
		}
		peerInfoByID := make(map[string]peerInfo, len(dbPeers))
		for _, p := range dbPeers {
			peerInfoByID[p.PeerID] = peerInfo{name: p.Name, ifaceName: p.Interface}
		}
		for _, wgPeer := range wgPeers {
			isOnline := isWgPeerOnline(wgPeer)
			if !isOnline {
				continue
			}
			wgOnlineCount++
			info := peerInfoByID[wgPeer.ID]
			name := info.name
			if name == "" {
				name = wgPeer.ID
			}
			users = append(users, schema.ReportsOnlineUser{
				Name:     name,
				Protocol: model.UsageProtocolWireGuard,
				Location: info.ifaceName,
			})
		}
	}

	var v2rayLocations []model.V2RayPackageLocation
	s.db.Where("is_online = ?", true).Find(&v2rayLocations)
	if len(v2rayLocations) > 0 {
		packageIDs := make([]uint, 0, len(v2rayLocations))
		panelIDs := make([]uint, 0, len(v2rayLocations))
		for _, loc := range v2rayLocations {
			packageIDs = append(packageIDs, loc.PackageID)
			panelIDs = append(panelIDs, loc.PanelID)
		}
		var packages []model.V2RayPackage
		s.db.Where("id IN ?", packageIDs).Find(&packages)
		packageLabel := make(map[uint]string, len(packages))
		for _, p := range packages {
			label := "N/A"
			if p.CustomerLabel != nil && *p.CustomerLabel != "" {
				label = *p.CustomerLabel
			}
			packageLabel[p.ID] = label
		}
		var panels []model.XuiPanel
		s.db.Where("id IN ?", panelIDs).Find(&panels)
		panelName := make(map[uint]string, len(panels))
		for _, p := range panels {
			panelName[p.ID] = p.Name
		}

		for _, loc := range v2rayLocations {
			users = append(users, schema.ReportsOnlineUser{
				Name:     packageLabel[loc.PackageID],
				Protocol: model.UsageProtocolV2Ray,
				Location: panelName[loc.PanelID],
			})
		}
	}

	return &schema.ReportsOnlineUsersResponse{
		TotalOnline: len(users),
		Users:       users,
	}, nil
}

// isWgPeerOnline mirrors WgPeer's own private handshakeData threshold
// (150s since last handshake) without depending on that unexported
// method -- WireGuardPeer.LastHandshake is a RouterOS-formatted duration
// string (e.g. "1m30s", "2h3m4s"); an empty/"0s"-ish value or a parse
// failure is treated as offline, matching the same fail-safe default used
// everywhere else a handshake can't be read.
func isWgPeerOnline(peer mikrotik.WireGuardPeer) bool {
	if peer.Disabled == "true" || peer.LastHandshake == nil || *peer.LastHandshake == "" {
		return false
	}
	duration, err := time.ParseDuration(*peer.LastHandshake)
	if err != nil {
		return false
	}
	return duration < 150*time.Second
}

// GetPanelHealth is report 9: every registered XuiPanel's own recent
// sync health -- a pure DB read (LastSyncedAt/LastSyncError are written
// by the periodic sync job, see V2RaySyncService.syncOneLocation), no
// live x-ui calls, matching V2RayPackageService.GetPackageShareDetails's
// own "never call x-ui at request time" rule.
func (s *ReportsLiveService) GetPanelHealth() (*schema.ReportsPanelHealthResponse, error) {
	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to fetch panels for health report", zap.Error(err))
		return nil, err
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations for panel health report", zap.Error(err))
		return nil, err
	}

	// reportPanelHealthMaxErrors caps how many individual error messages are
	// returned per panel -- the count (RecentErrorCount) is still exact,
	// this only bounds the payload/UI list size on a panel with many
	// failing locations, showing the most recent ones first.
	const reportPanelHealthMaxErrors = 20

	type panelAgg struct {
		locationCount int
		errorCount    int
		lastSyncedAt  *time.Time
		recentErrors  []string
	}
	aggByPanel := make(map[uint]*panelAgg, len(panels))
	for _, loc := range locations {
		agg, ok := aggByPanel[loc.PanelID]
		if !ok {
			agg = &panelAgg{}
			aggByPanel[loc.PanelID] = agg
		}
		agg.locationCount++
		if loc.LastSyncError != nil {
			agg.errorCount++
			if len(agg.recentErrors) < reportPanelHealthMaxErrors {
				agg.recentErrors = append(agg.recentErrors, *loc.LastSyncError)
			}
		}
		if loc.LastSyncedAt != nil && (agg.lastSyncedAt == nil || loc.LastSyncedAt.After(*agg.lastSyncedAt)) {
			agg.lastSyncedAt = loc.LastSyncedAt
		}
	}

	rows := make([]schema.ReportsPanelHealthRow, 0, len(panels))
	for _, p := range panels {
		agg := aggByPanel[p.ID]
		row := schema.ReportsPanelHealthRow{PanelID: p.ID, PanelName: p.Name}
		if agg != nil {
			row.LocationCount = agg.locationCount
			row.RecentErrorCount = agg.errorCount
			row.RecentErrors = agg.recentErrors
			if agg.lastSyncedAt != nil {
				formatted := agg.lastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
				row.LastSyncedAt = &formatted
			}
		}
		rows = append(rows, row)
	}

	return &schema.ReportsPanelHealthResponse{Rows: rows}, nil
}
