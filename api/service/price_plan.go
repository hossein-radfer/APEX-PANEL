package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

type PricePlan struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewPricePlan(db *gorm.DB) *PricePlan {
	return &PricePlan{
		db:     db,
		logger: zap.L().Named("PricePlanService"),
	}
}

// CreatePricePlan creates a new pricing plan
func (pp *PricePlan) CreatePricePlan(
	name string,
	description *string,
	basePriceAmount int64,
	billingInterval string,
	trafficAllowance *int64,
	maxPeers *int32,
	maxServers *int32,
) (*model.PricePlan, error) {
	if name == "" {
		return nil, errors.New("plan name is required")
	}

	if basePriceAmount < 0 {
		return nil, errors.New("base price must be non-negative")
	}

	plan := model.PricePlan{
		Name:             name,
		Description:      description,
		BasePriceAmount:  basePriceAmount,
		BillingInterval:  billingInterval,
		TrafficAllowance: trafficAllowance,
		MaxPeers:         maxPeers,
		MaxServers:       maxServers,
		IsActive:         true,
	}

	if err := pp.db.Create(&plan).Error; err != nil {
		pp.logger.Error("failed to create price plan", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	return &plan, nil
}

// GetPricePlan retrieves a pricing plan by ID
func (pp *PricePlan) GetPricePlan(planID uint) (*model.PricePlan, error) {
	var plan model.PricePlan
	if err := pp.db.First(&plan, "id = ? AND is_active = ?", planID, true).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		pp.logger.Error("failed to fetch price plan", zap.Uint("plan_id", planID), zap.Error(err))
		return nil, err
	}

	return &plan, nil
}

// ListActivePricePlans returns all active pricing plans
func (pp *PricePlan) ListActivePricePlans() ([]model.PricePlan, error) {
	var plans []model.PricePlan
	if err := pp.db.Where("is_active = ?", true).Order("name ASC").Find(&plans).Error; err != nil {
		pp.logger.Error("failed to list price plans", zap.Error(err))
		return nil, err
	}

	return plans, nil
}

// UpdatePricePlan updates an existing pricing plan
func (pp *PricePlan) UpdatePricePlan(
	planID uint,
	name *string,
	description *string,
	basePriceAmount *int64,
	billingInterval *string,
	trafficAllowance *int64,
	maxPeers *int32,
	maxServers *int32,
) (*model.PricePlan, error) {
	plan, err := pp.GetPricePlan(planID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if basePriceAmount != nil {
		if *basePriceAmount < 0 {
			return nil, errors.New("base price must be non-negative")
		}
		updates["base_price_amount"] = *basePriceAmount
	}
	if billingInterval != nil {
		updates["billing_interval"] = *billingInterval
	}
	if trafficAllowance != nil {
		updates["traffic_allowance"] = *trafficAllowance
	}
	if maxPeers != nil {
		updates["max_peers"] = *maxPeers
	}
	if maxServers != nil {
		updates["max_servers"] = *maxServers
	}
	updates["updated_at"] = time.Now()

	if err := pp.db.Model(&plan).Updates(updates).Error; err != nil {
		pp.logger.Error("failed to update price plan", zap.Uint("plan_id", planID), zap.Error(err))
		return nil, err
	}

	return plan, nil
}

// DeactivatePricePlan marks a pricing plan as inactive
func (pp *PricePlan) DeactivatePricePlan(planID uint) error {
	result := pp.db.Model(&model.PricePlan{}).Where("id = ?", planID).Update("is_active", false)
	if result.Error != nil {
		pp.logger.Error("failed to deactivate price plan", zap.Uint("plan_id", planID), zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
