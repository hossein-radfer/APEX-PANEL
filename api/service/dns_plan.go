package service

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// DNSPlanService manages the admin-defined DNS tier catalog (e.g.
// برنزی/نقره‌ای/طلایی) -- see model.DNSPlan's own doc comment for why this
// is a global catalog (mirroring TrafficPackageService's shape) rather than
// scoped to a specific DNSPanel or reseller.
type DNSPlanService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewDNSPlanService(db *gorm.DB) *DNSPlanService {
	return &DNSPlanService{
		db:     db,
		logger: zap.L().Named("DNSPlanService"),
	}
}

func (s *DNSPlanService) CreatePlan(req *schema.CreateDNSPlanRequest) (*schema.DNSPlanResponse, error) {
	plan := model.DNSPlan{
		Name:                     req.Name,
		Description:              req.Description,
		PriceAmount:              req.PriceAmount,
		TotalVolumeBytes:         req.TotalVolumeBytes,
		SpeedKbps:                req.SpeedKbps,
		DurationDays:             req.DurationDays,
		MaxConcurrentIPs:         req.MaxConcurrentIPs,
		DailyIPRegistrationLimit: req.DailyIPRegistrationLimit,
		IsActive:                 true,
	}

	if err := s.db.Create(&plan).Error; err != nil {
		s.logger.Error("failed to create DNS plan", zap.Error(err))
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	resp := transformDNSPlanToResponse(plan)
	return &resp, nil
}

func (s *DNSPlanService) UpdatePlan(id uint, req *schema.UpdateDNSPlanRequest) (*schema.DNSPlanResponse, error) {
	var plan model.DNSPlan
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
	if req.SpeedKbps != nil {
		plan.SpeedKbps = *req.SpeedKbps
	}
	if req.DurationDays != nil {
		plan.DurationDays = *req.DurationDays
	}
	if req.MaxConcurrentIPs != nil {
		plan.MaxConcurrentIPs = *req.MaxConcurrentIPs
	}
	if req.DailyIPRegistrationLimit != nil {
		plan.DailyIPRegistrationLimit = *req.DailyIPRegistrationLimit
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}

	if err := s.db.Save(&plan).Error; err != nil {
		s.logger.Error("failed to update DNS plan", zap.Error(err))
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	resp := transformDNSPlanToResponse(plan)
	return &resp, nil
}

// DeletePlan removes the catalog entry only -- any DNSAccount still
// referencing this plan's id keeps its own already-copied field values
// unchanged (DNSAccount.DNSPlanID simply becomes a dangling reference,
// mirroring TrafficPackage/PackagePurchase's identical "delete the catalog
// row, historical/already-applied records are unaffected" convention).
func (s *DNSPlanService) DeletePlan(id uint) error {
	if err := s.db.Delete(&model.DNSPlan{}, id).Error; err != nil {
		s.logger.Error("failed to delete DNS plan", zap.Error(err))
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	return nil
}

func (s *DNSPlanService) ListPlans() ([]schema.DNSPlanResponse, error) {
	var plans []model.DNSPlan
	if err := s.db.Order("price_amount asc").Find(&plans).Error; err != nil {
		s.logger.Error("failed to list DNS plans", zap.Error(err))
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}

	resp := make([]schema.DNSPlanResponse, 0, len(plans))
	for _, p := range plans {
		resp = append(resp, transformDNSPlanToResponse(p))
	}
	return resp, nil
}

// GetPlan is used internally by DNSAccountService when an account create/
// update request references a plan, to copy its bundle into the account's
// own fields.
func (s *DNSPlanService) GetPlan(id uint) (*model.DNSPlan, error) {
	var plan model.DNSPlan
	if err := s.db.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func transformDNSPlanToResponse(p model.DNSPlan) schema.DNSPlanResponse {
	return schema.DNSPlanResponse{
		Id:                       p.ID,
		Name:                     p.Name,
		Description:              p.Description,
		PriceAmount:              p.PriceAmount,
		TotalVolumeBytes:         p.TotalVolumeBytes,
		SpeedKbps:                p.SpeedKbps,
		DurationDays:             p.DurationDays,
		MaxConcurrentIPs:         p.MaxConcurrentIPs,
		DailyIPRegistrationLimit: p.DailyIPRegistrationLimit,
		IsActive:                 p.IsActive,
	}
}
