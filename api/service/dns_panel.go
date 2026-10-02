package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/doctordns"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// DNSPanelService manages admin-registered doctor-dns installations -- pure
// DB CRUD plus a synchronous connection test, mirroring XuiPanelService's
// identical role for x-ui panels.
type DNSPanelService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewDNSPanelService(db *gorm.DB) *DNSPanelService {
	return &DNSPanelService{
		db:     db,
		logger: zap.L().Named("DNSPanelService"),
	}
}

func (s *DNSPanelService) CreatePanel(req *schema.CreateDNSPanelRequest) (*schema.DNSPanelResponse, error) {
	panel := model.DNSPanel{
		Name:       req.Name,
		SaleTitle:  req.SaleTitle,
		APIBaseURL: req.APIBaseURL,
		APIKey:     req.APIKey,
		Status:     "active",
	}

	if err := s.db.Create(&panel).Error; err != nil {
		s.logger.Error("failed to create DNS panel", zap.Error(err))
		return nil, fmt.Errorf("failed to create panel: %w", err)
	}

	resp := transformDNSPanelToResponse(panel)
	return &resp, nil
}

func (s *DNSPanelService) UpdatePanel(id uint, req *schema.UpdateDNSPanelRequest) (*schema.DNSPanelResponse, error) {
	var panel model.DNSPanel
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
	if req.APIKey != nil && *req.APIKey != "" {
		panel.APIKey = *req.APIKey
	}

	if err := s.db.Save(&panel).Error; err != nil {
		s.logger.Error("failed to update DNS panel", zap.Error(err))
		return nil, fmt.Errorf("failed to update panel: %w", err)
	}

	resp := transformDNSPanelToResponse(panel)
	return &resp, nil
}

// DeletePanel removes the panel registration only -- mirrors
// XuiPanelService.DeletePanel's identical "does not fan out to existing
// dependents" behavior. Any DNSAccount still pointing at this panel is left
// in place with a permanently-stale LastSyncError once the next sync tick
// can no longer resolve it.
func (s *DNSPanelService) DeletePanel(id uint) error {
	if err := s.db.Delete(&model.DNSPanel{}, id).Error; err != nil {
		s.logger.Error("failed to delete DNS panel", zap.Error(err))
		return fmt.Errorf("failed to delete panel: %w", err)
	}
	return nil
}

func (s *DNSPanelService) ListPanels() ([]schema.DNSPanelResponse, error) {
	var panels []model.DNSPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to list DNS panels", zap.Error(err))
		return nil, fmt.Errorf("failed to list panels: %w", err)
	}

	resp := make([]schema.DNSPanelResponse, 0, len(panels))
	for _, p := range panels {
		resp = append(resp, transformDNSPanelToResponse(p))
	}
	return resp, nil
}

// GetPanel is used internally by DNSAccountService/the sync job to resolve a
// panel row -- not exposed as its own HTTP endpoint.
func (s *DNSPanelService) GetPanel(id uint) (model.DNSPanel, error) {
	var panel model.DNSPanel
	err := s.db.First(&panel, id).Error
	return panel, err
}

// ListAllPanelModels returns the raw models (not DTOs) for every registered
// panel -- used by DNSAccountService, which needs the plaintext APIKey the
// response DTO deliberately omits.
func (s *DNSPanelService) ListAllPanelModels() ([]model.DNSPanel, error) {
	var panels []model.DNSPanel
	if err := s.db.Find(&panels).Error; err != nil {
		return nil, err
	}
	return panels, nil
}

// TestConnection fetches panel's (by ID, using its already-stored
// credentials) template list in one synchronous round-trip -- the admin's
// "Test Connection" button, which also populates the plan-picker dropdown
// from the same call. On success marks Status="active"/clears LastError; on
// failure marks Status="error"/records LastError, mirroring what the
// background health job does on an ongoing basis.
func (s *DNSPanelService) TestConnection(id uint) (*schema.TestDNSConnectionResponse, error) {
	panel, err := s.GetPanel(id)
	if err != nil {
		return nil, fmt.Errorf("panel not found: %w", err)
	}

	templates, err := doctordns.ListTemplates(context.Background(), panel)
	if err != nil {
		s.recordPanelHealth(panel.ID, err)
		return nil, err
	}
	s.recordPanelHealth(panel.ID, nil)

	return &schema.TestDNSConnectionResponse{Templates: transformTemplates(templates)}, nil
}

// TestConnectionUnsaved is used by the create-panel dialog to test
// connectivity BEFORE the panel has been persisted, using the form's
// in-progress field values directly.
func (s *DNSPanelService) TestConnectionUnsaved(req *schema.TestDNSPanelConnectionRequest) (*schema.TestDNSConnectionResponse, error) {
	panel := model.DNSPanel{
		APIBaseURL: req.APIBaseURL,
		APIKey:     req.APIKey,
	}

	templates, err := doctordns.ListTemplates(context.Background(), panel)
	if err != nil {
		return nil, err
	}

	return &schema.TestDNSConnectionResponse{Templates: transformTemplates(templates)}, nil
}

func transformTemplates(templates []doctordns.Template) []schema.DNSTemplateOption {
	options := make([]schema.DNSTemplateOption, 0, len(templates))
	for _, t := range templates {
		options = append(options, schema.DNSTemplateOption{Id: t.ID, Name: t.Name, IsDefault: t.IsDefault})
	}
	return options
}

// recordPanelHealth mirrors what the background health job also writes on
// every tick -- factored out so a manual "Test Connection" click updates the
// same Status/LastError fields the admin panel list displays.
func (s *DNSPanelService) recordPanelHealth(panelID uint, syncErr error) {
	updates := map[string]interface{}{}
	if syncErr != nil {
		updates["status"] = "error"
		updates["last_error"] = syncErr.Error()
	} else {
		updates["status"] = "active"
		updates["last_error"] = nil
	}

	if err := s.db.Model(&model.DNSPanel{}).Where("id = ?", panelID).Updates(updates).Error; err != nil {
		s.logger.Warn("failed to record panel health", zap.Uint("panel_id", panelID), zap.Error(err))
	}
}

func transformDNSPanelToResponse(panel model.DNSPanel) schema.DNSPanelResponse {
	var lastSyncedAt *string
	if panel.LastSyncedAt != nil {
		formatted := panel.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
		lastSyncedAt = &formatted
	}

	return schema.DNSPanelResponse{
		Id:           panel.ID,
		Name:         panel.Name,
		SaleTitle:    panel.SaleTitle,
		APIBaseURL:   panel.APIBaseURL,
		Status:       panel.Status,
		LastError:    panel.LastError,
		LastSyncedAt: lastSyncedAt,
	}
}
