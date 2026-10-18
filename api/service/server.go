package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Server struct {
	db              *gorm.DB
	mwpClients      *common.MwpClients
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewServerService(db *gorm.DB, mwpClients *common.MwpClients, mikrotikAdaptor *mikrotik.Adaptor) *Server {
	return &Server{
		db:              db,
		mwpClients:      mwpClients,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("ServerService"),
	}
}

func (s *Server) ToggleServerStatus(id uint) (*schema.ServerResponse, error) {
	var server model.Server
	if err := s.db.First(&server, id).Error; err != nil {
		s.logger.Error("failed to find server by ID", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}

	server.IsActive = !server.IsActive

	if err := s.db.Save(&server).Error; err != nil {
		s.logger.Error("failed to update server status", zap.Error(err))
		return nil, err
	}

	return &schema.ServerResponse{
		Id:        server.ID,
		Comment:   server.Comment,
		Name:      server.Name,
		IPAddress: server.IPAddress,
		APIPort:   strconv.Itoa(server.APIPort),
		IsActive:  server.IsActive,
		Status:    schema.AvailableServer,
	}, nil
}

func (s *Server) CreateServer(req *schema.CreateServerRequest) (*schema.ServerResponse, error) {
	apiPort, err := strconv.Atoi(req.APIPort)
	if err != nil {
		s.logger.Error("invalid API port", zap.Error(err))
		return nil, err
	}

	isSSL := req.IsSSL != nil && *req.IsSSL

	server := model.Server{
		Comment:   req.Comment,
		Name:      req.Name,
		IPAddress: req.IPAddress,
		APIPort:   apiPort,
		IsSSL:     isSSL,
		Username:  req.Username,
		Password:  req.Password,
	}

	// The DB row is created FIRST, and SetClient/the validation probe both
	// use its real, final ID -- a confirmed, reported lifecycle bug this
	// fixes: the old order called SetClient (keyed by Name at the time)
	// before the row existed, so the validation probe and the client that
	// ended up registered for this server's eventual ID were never
	// guaranteed to be the same connection config if CreateServer failed
	// partway and was retried with different credentials.
	if err := s.db.Create(&server).Error; err != nil {
		s.logger.Error("failed to create server record", zap.Error(err))
		return nil, err
	}

	s.mwpClients.SetClient(server.ID, req)

	if _, err := s.mikrotikAdaptor.ForServer(server.ID).FetchDeviceIdentity(context.Background()); err != nil {
		s.logger.Error("failed to connect to device when creating a new server", zap.Error(err))
		// The row and its client were already created above -- roll both
		// back rather than leaving an unreachable server registered, since
		// the caller expects CreateServer to either fully succeed or have
		// no effect.
		s.mwpClients.DeleteClient(server.ID)
		if delErr := s.db.Unscoped().Delete(&server).Error; delErr != nil {
			s.logger.Error("failed to roll back server record after failed connection probe", zap.Error(delErr))
		}
		return nil, err
	}

	return &schema.ServerResponse{
		Id:        server.ID,
		Comment:   server.Comment,
		Name:      server.Name,
		IPAddress: server.IPAddress,
		APIPort:   strconv.Itoa(server.APIPort),
		IsSSL:     server.IsSSL,
		IsActive:  server.IsActive,
		Status:    schema.AvailableServer,
	}, nil
}

func (s *Server) GetServers() (*[]schema.ServerResponse, error) {
	var servers []model.Server
	if err := s.db.Find(&servers).Error; err != nil {
		s.logger.Error("failed to fetch servers from database", zap.Error(err))
		return nil, err
	}

	var serverResponses []schema.ServerResponse
	for _, server := range servers {
		serverStatus := schema.AvailableServer
		// CheckConnection, keyed by THIS row's own ID, replaces the old
		// FetchDeviceIdentity(context.Background()) call here -- that call
		// went through the adaptor's then-only "whichever client the map
		// returns first" resolution, so every server in this list reported
		// the exact same status regardless of which one was actually being
		// rendered. CheckConnection also gives a richer health signal
		// (reason/detail) than a bare connect-or-not probe, already used
		// elsewhere for this exact purpose (GetConnectionHealth).
		if connected, _, _ := s.mwpClients.CheckConnection(&server.ID); !connected {
			serverStatus = schema.NotAvailableServer
		}

		serverResponses = append(serverResponses, schema.ServerResponse{
			Id:        server.ID,
			Comment:   server.Comment,
			Name:      server.Name,
			IPAddress: server.IPAddress,
			APIPort:   strconv.Itoa(server.APIPort),
			IsSSL:     server.IsSSL,
			IsActive:  server.IsActive,
			Status:    serverStatus,
		})
	}

	return &serverResponses, nil
}

// GetServerEndpoints returns minimal connection info (no credentials) for all
// servers. Safe to expose to any authenticated role, including resellers, who
// need a server's IP address to build a WireGuard peer config but must not
// see or manage the full server list.
func (s *Server) GetServerEndpoints() (*[]schema.ServerEndpointResponse, error) {
	var servers []model.Server
	if err := s.db.Find(&servers).Error; err != nil {
		s.logger.Error("failed to fetch servers from database", zap.Error(err))
		return nil, err
	}

	endpoints := make([]schema.ServerEndpointResponse, 0, len(servers))
	for _, server := range servers {
		endpoints = append(endpoints, schema.ServerEndpointResponse{
			Id:        server.ID,
			Name:      server.Name,
			IPAddress: server.IPAddress,
		})
	}

	return &endpoints, nil
}

func (s *Server) UpdateServer(id uint, req *schema.UpdateServerRequest) (*schema.ServerResponse, error) {
	var server model.Server
	if err := s.db.First(&server, id).Error; err != nil {
		s.logger.Error("failed to find server by ID", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}

	server.Comment = req.Comment
	server.Name = *req.Name
	server.IPAddress = *req.IPAddress

	apiPort, err := strconv.Atoi(*req.APIPort)
	if err != nil {
		s.logger.Error("invalid API port", zap.Error(err))
		return nil, err
	}
	server.APIPort = apiPort

	if req.IsSSL != nil {
		server.IsSSL = *req.IsSSL
	}

	server.Username = *req.Username
	server.Password = *req.Password

	if err := s.db.Save(&server).Error; err != nil {
		s.logger.Error("failed to update server record", zap.Error(err))
		return nil, err
	}

	// Re-register this server's client immediately so an edited IP/
	// credentials/SSL setting take effect right away -- a confirmed,
	// reported lifecycle bug this fixes: before this call existed, an
	// edited server's in-memory client kept using its OLD connection
	// config until the next full panel restart (InitClient is the only
	// other place a client gets (re)built), silently leaving every
	// WireGuard/UserManager/tunnel-health call against this server pointed
	// at stale credentials or a stale IP in the meantime.
	isSSL := server.IsSSL
	s.mwpClients.SetClient(server.ID, &schema.CreateServerRequest{
		Name:      server.Name,
		IPAddress: server.IPAddress,
		APIPort:   strconv.Itoa(server.APIPort),
		IsSSL:     &isSSL,
		Username:  server.Username,
		Password:  server.Password,
	})

	return &schema.ServerResponse{
		Id:        server.ID,
		Comment:   server.Comment,
		Name:      server.Name,
		IPAddress: server.IPAddress,
		APIPort:   strconv.Itoa(server.APIPort),
		IsSSL:     server.IsSSL,
		IsActive:  server.IsActive,
		Status:    schema.AvailableServer,
	}, nil
}

func (s *Server) DeleteServer(id uint) error {
	var server model.Server
	if err := s.db.First(&server, id).Error; err != nil {
		s.logger.Error("failed to find server by ID", zap.Uint("id", id), zap.Error(err))
		return err
	}

	// Refuse to delete a server that still owns Peer/Interface/
	// UserManagerAccount rows -- there is no database-level FK enforcing
	// this relationship (ServerID is a plain uint column, not a declared
	// foreign key), so without this guard a delete here would silently
	// orphan those rows: their ServerID would point at a server that no
	// longer exists, and every adaptor call derived from them (ForServer)
	// would fail with "no mikrotik client configured" from then on with no
	// clear indication why.
	if blockers, err := s.countServerDependents(id); err != nil {
		s.logger.Error("failed to check for dependent rows before deleting server", zap.Error(err))
		return err
	} else if blockers != "" {
		return fmt.Errorf("cannot delete server %q: it still has %s -- move or remove them first", server.Name, blockers)
	}

	if err := s.db.Unscoped().Delete(&server).Error; err != nil {
		s.logger.Error("failed to delete server record", zap.Error(err))
		return err
	}

	s.mwpClients.DeleteClient(server.ID)

	return nil
}

// countServerDependents returns a short, human-readable description of
// which dependent row types (Peer/Interface/UserManagerAccount) still
// reference serverID, or "" if there are none -- used by DeleteServer's own
// guard against orphaning rows that have no database-level FK back to
// Server (see that call site's own doc comment).
func (s *Server) countServerDependents(serverID uint) (string, error) {
	var peerCount, ifaceCount, accountCount int64

	if err := s.db.Model(&model.Peer{}).Where("server_id = ?", serverID).Count(&peerCount).Error; err != nil {
		return "", err
	}
	if err := s.db.Model(&model.Interface{}).Where("server_id = ?", serverID).Count(&ifaceCount).Error; err != nil {
		return "", err
	}
	if err := s.db.Model(&model.UserManagerAccount{}).Where("server_id = ?", serverID).Count(&accountCount).Error; err != nil {
		return "", err
	}

	var parts []string
	if ifaceCount > 0 {
		parts = append(parts, fmt.Sprintf("%d interface(s)", ifaceCount))
	}
	if peerCount > 0 {
		parts = append(parts, fmt.Sprintf("%d peer(s)", peerCount))
	}
	if accountCount > 0 {
		parts = append(parts, fmt.Sprintf("%d user manager account(s)", accountCount))
	}
	if len(parts) == 0 {
		return "", nil
	}

	result := parts[0]
	for _, p := range parts[1:] {
		result += ", " + p
	}
	return result, nil
}

func (s *Server) GetServersData() (*schema.ServerStatsResponse, error) {
	var totalServers int64
	if err := s.db.Model(&model.Server{}).Count(&totalServers).Error; err != nil {
		s.logger.Error("failed to count total servers", zap.Error(err))
		return nil, err
	}

	var activeServers int64
	if err := s.db.Model(&model.Server{}).Where("is_active = ?", true).Count(&activeServers).Error; err != nil {
		s.logger.Error("failed to count active servers", zap.Error(err))
		return nil, err
	}

	return &schema.ServerStatsResponse{
		TotalServers:  int(totalServers),
		ActiveServers: int(activeServers),
	}, nil
}
