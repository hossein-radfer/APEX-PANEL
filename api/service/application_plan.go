package service

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// ApplicationPlanService manages the admin-defined Application tier catalog
// -- mirrors DNSPlanService's shape exactly (see model.ApplicationPlan's
// own doc comment for why this is a global catalog).
type ApplicationPlanService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewApplicationPlanService(db *gorm.DB) *ApplicationPlanService {
	return &ApplicationPlanService{
		db:     db,
		logger: zap.L().Named("ApplicationPlanService"),
	}
}

func (s *ApplicationPlanService) CreatePlan(req *schema.CreateApplicationPlanRequest) (*schema.ApplicationPlanResponse, error) {
	plan := model.ApplicationPlan{
		Name:                   req.Name,
		Description:            req.Description,
		PriceAmount:            req.PriceAmount,
		TotalVolumeBytes:       req.TotalVolumeBytes,
		DurationDays:           req.DurationDays,
		MaxOnlineUsers:         req.MaxOnlineUsers,
		DownloadSpeedLimitMbps: req.DownloadSpeedLimitMbps,
		UploadSpeedLimitMbps:   req.UploadSpeedLimitMbps,
		IsActive:               true,
	}

	if err := s.db.Create(&plan).Error; err != nil {
		s.logger.Error("failed to create application plan", zap.Error(err))
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	resp := transformApplicationPlanToResponse(plan)
	return &resp, nil
}

func (s *ApplicationPlanService) UpdatePlan(id uint, req *schema.UpdateApplicationPlanRequest) (*schema.ApplicationPlanResponse, error) {
	var plan model.ApplicationPlan
	if err := s.db.First(&plan, id).Error; err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.Description != nil {
		plan.Description = req.Description
	}
	if req.PriceAmount != nil {
		plan.PriceAmount = *req.PriceAmount
	}
	if req.TotalVolumeBytes != nil {
		plan.TotalVolumeBytes = *req.TotalVolumeBytes
	}
	if req.DurationDays != nil {
		plan.DurationDays = *req.DurationDays
	}
	if req.MaxOnlineUsers != nil {
		plan.MaxOnlineUsers = *req.MaxOnlineUsers
	}
	// ClearSpeedLimits takes precedence over a same-request explicit value
	// -- mirrors UpdateResellerRequest.ClearDNSMaxAccounts' own "explicit
	// clear wins" convention, since *int fields cannot otherwise
	// distinguish "leave unchanged" from "set to nil" over JSON.
	if req.ClearSpeedLimits != nil && *req.ClearSpeedLimits {
		plan.DownloadSpeedLimitMbps = nil
		plan.UploadSpeedLimitMbps = nil
	} else {
		if req.DownloadSpeedLimitMbps != nil {
			plan.DownloadSpeedLimitMbps = req.DownloadSpeedLimitMbps
		}
		if req.UploadSpeedLimitMbps != nil {
			plan.UploadSpeedLimitMbps = req.UploadSpeedLimitMbps
		}
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}

	if err := s.db.Save(&plan).Error; err != nil {
		s.logger.Error("failed to update application plan", zap.Error(err))
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	resp := transformApplicationPlanToResponse(plan)
	return &resp, nil
}

// DeletePlan removes the catalog entry only -- any Application still
// referencing this plan's id keeps its own already-copied field values
// unchanged, mirroring DNSPlanService.DeletePlan's identical convention.
func (s *ApplicationPlanService) DeletePlan(id uint) error {
	if err := s.db.Delete(&model.ApplicationPlan{}, id).Error; err != nil {
		s.logger.Error("failed to delete application plan", zap.Error(err))
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	return nil
}

// ListActivePlans is reachable by both admin and reseller sessions
// (mirrors TrafficPackageService.ListActiveTrafficPackages) -- a reseller
// creating an Application needs to see and pick from the active plan
// catalog exactly like they already can for traffic packages, unlike
// DNSPlanService.ListPlans, which is admin-only and therefore invisible to
// a reseller's own DNS account form.
func (s *ApplicationPlanService) ListActivePlans() ([]schema.ApplicationPlanResponse, error) {
	var plans []model.ApplicationPlan
	if err := s.db.Where("is_active = ?", true).Order("price_amount asc").Find(&plans).Error; err != nil {
		s.logger.Error("failed to list active application plans", zap.Error(err))
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}

	resp := make([]schema.ApplicationPlanResponse, 0, len(plans))
	for _, p := range plans {
		resp = append(resp, transformApplicationPlanToResponse(p))
	}
	return resp, nil
}

// ListAllPlans returns every plan regardless of active status -- admin-only,
// used by the plan management page so a disabled plan can still be found
// and re-enabled/edited.
func (s *ApplicationPlanService) ListAllPlans() ([]schema.ApplicationPlanResponse, error) {
	var plans []model.ApplicationPlan
	if err := s.db.Order("price_amount asc").Find(&plans).Error; err != nil {
		s.logger.Error("failed to list all application plans", zap.Error(err))
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}

	resp := make([]schema.ApplicationPlanResponse, 0, len(plans))
	for _, p := range plans {
		resp = append(resp, transformApplicationPlanToResponse(p))
	}
	return resp, nil
}

// GetPlan is used internally by ApplicationService when a create request
// references a plan, to copy its bundle into the application's own fields.
func (s *ApplicationPlanService) GetPlan(id uint) (*model.ApplicationPlan, error) {
	var plan model.ApplicationPlan
	if err := s.db.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func transformApplicationPlanToResponse(p model.ApplicationPlan) schema.ApplicationPlanResponse {
	return schema.ApplicationPlanResponse{
		Id:                     p.ID,
		Name:                   p.Name,
		Description:            p.Description,
		PriceAmount:            p.PriceAmount,
		TotalVolumeBytes:       p.TotalVolumeBytes,
		DurationDays:           p.DurationDays,
		MaxOnlineUsers:         p.MaxOnlineUsers,
		DownloadSpeedLimitMbps: p.DownloadSpeedLimitMbps,
		UploadSpeedLimitMbps:   p.UploadSpeedLimitMbps,
		IsActive:               p.IsActive,
	}
}
