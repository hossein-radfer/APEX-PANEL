package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/xui"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// XuiPanelService manages admin-registered x-ui panels -- pure DB CRUD plus
// a synchronous connection test, all admin-only (enforced at the HTTP
// layer, matching ResellerController's convention). No caching of any
// login/session state, matching the xui adaptor package's own "fresh login
// every operation" design.
type XuiPanelService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewXuiPanelService(db *gorm.DB) *XuiPanelService {
	return &XuiPanelService{
		db:     db,
		logger: zap.L().Named("XuiPanelService"),
	}
}

func (s *XuiPanelService) CreatePanel(req *schema.CreateXuiPanelRequest) (*schema.XuiPanelResponse, error) {
	panel := model.XuiPanel{
		Name:              req.Name,
		SaleTitle:         req.SaleTitle,
		APIBaseURL:        req.APIBaseURL,
		Username:          req.Username,
		Password:          req.Password,
		DefaultInboundID:  req.DefaultInboundID,
		Protocol:          req.Protocol,
		SubBaseURL:        req.SubBaseURL,
		Status:            "active",
		ContainerServerID: req.ContainerServerID,
		ContainerName:     req.ContainerName,
	}

	if err := s.db.Create(&panel).Error; err != nil {
		s.logger.Error("failed to create x-ui panel", zap.Error(err))
		return nil, fmt.Errorf("failed to create panel: %w", err)
	}

	return s.transformPanelToResponseWithServerName(panel)
}

func (s *XuiPanelService) UpdatePanel(id uint, req *schema.UpdateXuiPanelRequest) (*schema.XuiPanelResponse, error) {
	var panel model.XuiPanel
	if err := s.db.First(&panel, id).Error; err != nil {
		return nil, fmt.Errorf("panel not found: %w", err)
	}

	if req.Name != "" {
		panel.Name = req.Name
	}
	if req.SaleTitle != "" {
		panel.SaleTitle = req.SaleTitle
	}
	if req.APIBaseURL != "" {
		panel.APIBaseURL = req.APIBaseURL
	}
	if req.Username != "" {
		panel.Username = req.Username
	}
	if req.Password != nil && *req.Password != "" {
		panel.Password = *req.Password
	}
	if req.DefaultInboundID != 0 {
		panel.DefaultInboundID = req.DefaultInboundID
	}
	if req.Protocol != "" {
		panel.Protocol = req.Protocol
	}
	if req.SubBaseURL != "" {
		panel.SubBaseURL = req.SubBaseURL
	}

	if req.ClearContainerMapping != nil && *req.ClearContainerMapping {
		panel.ContainerServerID = nil
		panel.ContainerName = nil
	} else {
		if req.ContainerServerID != nil {
			panel.ContainerServerID = req.ContainerServerID
		}
		if req.ContainerName != nil {
			panel.ContainerName = req.ContainerName
		}
	}

	if err := s.db.Save(&panel).Error; err != nil {
		s.logger.Error("failed to update x-ui panel", zap.Error(err))
		return nil, fmt.Errorf("failed to update panel: %w", err)
	}

	return s.transformPanelToResponseWithServerName(panel)
}

// DeletePanel removes the panel registration only -- it deliberately does
// NOT fan out delete calls to every V2RayPackageLocation still pointing at
// it (that would require live-calling a panel the admin may be removing
// precisely because it's gone/decommissioned). Existing locations on this
// panel are left in place with a now-permanently-stale LastSyncError from
// the next sync tick (the panel row they reference no longer resolves),
// which is surfaced in the admin UI rather than silently hidden.
func (s *XuiPanelService) DeletePanel(id uint) error {
	if err := s.db.Delete(&model.XuiPanel{}, id).Error; err != nil {
		s.logger.Error("failed to delete x-ui panel", zap.Error(err))
		return fmt.Errorf("failed to delete panel: %w", err)
	}
	return nil
}

func (s *XuiPanelService) ListPanels() ([]schema.XuiPanelResponse, error) {
	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to list x-ui panels", zap.Error(err))
		return nil, fmt.Errorf("failed to list panels: %w", err)
	}

	// Batch-resolve every referenced Server's Name in one query rather than
	// one lookup per panel -- mirrors ListInstancesWithResellerNames's own
	// "join in the service layer, not per-row" convention elsewhere in
	// this codebase.
	serverIDs := make([]uint, 0, len(panels))
	for _, p := range panels {
		if p.ContainerServerID != nil {
			serverIDs = append(serverIDs, *p.ContainerServerID)
		}
	}
	serverNames := make(map[uint]string, len(serverIDs))
	if len(serverIDs) > 0 {
		var servers []model.Server
		if err := s.db.Where("id IN ?", serverIDs).Find(&servers).Error; err != nil {
			s.logger.Warn("failed to resolve container server names", zap.Error(err))
		} else {
			for _, srv := range servers {
				serverNames[srv.ID] = srv.Name
			}
		}
	}

	resp := make([]schema.XuiPanelResponse, 0, len(panels))
	for _, p := range panels {
		item := transformPanelToResponse(p)
		if p.ContainerServerID != nil {
			if name, ok := serverNames[*p.ContainerServerID]; ok {
				item.ContainerServerName = &name
			}
		}
		resp = append(resp, item)
	}
	return resp, nil
}

// GetPanel is used internally by V2RayPackageService/the sync job to
// resolve a panel row -- not exposed as its own HTTP endpoint (ListPanels
// covers the admin UI's needs).
func (s *XuiPanelService) GetPanel(id uint) (model.XuiPanel, error) {
	var panel model.XuiPanel
	err := s.db.First(&panel, id).Error
	return panel, err
}

// ListAllPanelModels returns the raw models (not DTOs) for every registered
// panel -- used by V2RayPackageService's fan-out create/delete/update paths,
// which need the plaintext Password/APIBaseURL fields the response DTO
// deliberately omits.
func (s *XuiPanelService) ListAllPanelModels() ([]model.XuiPanel, error) {
	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		return nil, err
	}
	return panels, nil
}

// TestConnection logs into panel (by ID, using its already-stored
// credentials) and fetches its inbound list in one synchronous round-trip
// -- used both by the admin's "Test Connection" button (which populates
// the DefaultInboundID dropdown from the same call) and, on success, marks
// the panel Status="active"/clears LastError; on failure marks
// Status="error"/records LastError, mirroring what the background sync job
// does on an ongoing basis.
func (s *XuiPanelService) TestConnection(id uint) (*schema.TestXuiConnectionResponse, error) {
	panel, err := s.GetPanel(id)
	if err != nil {
		return nil, fmt.Errorf("panel not found: %w", err)
	}

	inbounds, err := xui.ListInbounds(context.Background(), panel)
	if err != nil {
		s.recordPanelHealth(panel.ID, err)
		return nil, err
	}
	s.recordPanelHealth(panel.ID, nil)

	options := make([]schema.XuiInboundOption, 0, len(inbounds))
	for _, i := range inbounds {
		options = append(options, schema.XuiInboundOption{Id: i.ID, Remark: i.Remark, Protocol: i.Protocol})
	}

	return &schema.TestXuiConnectionResponse{Inbounds: options}, nil
}

// TestConnectionUnsaved is used by the create-panel dialog to test
// connectivity BEFORE the panel has been persisted, using the form's
// in-progress field values directly.
func (s *XuiPanelService) TestConnectionUnsaved(req *schema.TestXuiPanelConnectionRequest) (*schema.TestXuiConnectionResponse, error) {
	panel := model.XuiPanel{
		APIBaseURL: req.APIBaseURL,
		Username:   req.Username,
		Password:   req.Password,
	}

	inbounds, err := xui.ListInbounds(context.Background(), panel)
	if err != nil {
		return nil, err
	}

	options := make([]schema.XuiInboundOption, 0, len(inbounds))
	for _, i := range inbounds {
		options = append(options, schema.XuiInboundOption{Id: i.ID, Remark: i.Remark, Protocol: i.Protocol})
	}

	return &schema.TestXuiConnectionResponse{Inbounds: options}, nil
}

// recordPanelHealth mirrors what the background sync job also writes on
// every tick -- factored out so a manual "Test Connection" click updates
// the same Status/LastError fields the admin panel list displays.
func (s *XuiPanelService) recordPanelHealth(panelID uint, syncErr error) {
	updates := map[string]interface{}{}
	if syncErr != nil {
		updates["status"] = "error"
		errStr := syncErr.Error()
		updates["last_error"] = errStr
	} else {
		updates["status"] = "active"
		updates["last_error"] = nil
	}

	if err := s.db.Model(&model.XuiPanel{}).Where("id = ?", panelID).Updates(updates).Error; err != nil {
		s.logger.Warn("failed to record panel health", zap.Uint("panel_id", panelID), zap.Error(err))
	}
}

func transformPanelToResponse(panel model.XuiPanel) schema.XuiPanelResponse {
	var lastSyncedAt *string
	if panel.LastSyncedAt != nil {
		formatted := panel.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
		lastSyncedAt = &formatted
	}

	return schema.XuiPanelResponse{
		Id:                panel.ID,
		Name:              panel.Name,
		SaleTitle:         panel.SaleTitle,
		APIBaseURL:        panel.APIBaseURL,
		Username:          panel.Username,
		DefaultInboundID:  panel.DefaultInboundID,
		Protocol:          panel.Protocol,
		SubBaseURL:        panel.SubBaseURL,
		Status:            panel.Status,
		LastError:         panel.LastError,
		LastSyncedAt:      lastSyncedAt,
		ContainerServerID: panel.ContainerServerID,
		ContainerName:     panel.ContainerName,
	}
}

// transformPanelToResponseWithServerName is CreatePanel/UpdatePanel's own
// single-panel counterpart to ListPanels' batch server-name resolution --
// a single extra lookup here is fine (unlike ListPanels, this isn't in a
// per-row loop over potentially many panels).
func (s *XuiPanelService) transformPanelToResponseWithServerName(panel model.XuiPanel) (*schema.XuiPanelResponse, error) {
	resp := transformPanelToResponse(panel)
	if panel.ContainerServerID != nil {
		var srv model.Server
		if err := s.db.First(&srv, *panel.ContainerServerID).Error; err == nil {
			resp.ContainerServerName = &srv.Name
		}
	}
	return &resp, nil
}
