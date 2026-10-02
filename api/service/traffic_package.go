package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TrafficPackageService manages the admin-defined catalog of one-time extra
// traffic packages a reseller can buy when their main quota runs out. This
// is the single source of truth both the web panel and the Telegram bot read
// from, so a package's price/size is always identical in both places.
type TrafficPackageService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewTrafficPackageService(db *gorm.DB) *TrafficPackageService {
	return &TrafficPackageService{
		db:     db,
		logger: zap.L().Named("TrafficPackageService"),
	}
}

func (s *TrafficPackageService) CreateTrafficPackage(name string, description *string, trafficBytes int64, priceAmount int64) (*model.TrafficPackage, error) {
	if name == "" {
		return nil, errors.New("package name is required")
	}
	if trafficBytes <= 0 {
		return nil, errors.New("traffic size must be positive")
	}
	if priceAmount < 0 {
		return nil, errors.New("price must be non-negative")
	}

	pkg := model.TrafficPackage{
		Name:         name,
		Description:  description,
		TrafficBytes: trafficBytes,
		PriceAmount:  priceAmount,
		IsActive:     true,
	}

	if err := s.db.Create(&pkg).Error; err != nil {
		s.logger.Error("failed to create traffic package", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

// GetTrafficPackage returns an active package by ID. Deactivated packages
// are treated as not-found, since they must no longer be purchasable.
func (s *TrafficPackageService) GetTrafficPackage(id uint) (*model.TrafficPackage, error) {
	var pkg model.TrafficPackage
	if err := s.db.First(&pkg, "id = ? AND is_active = ?", id, true).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch traffic package", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	return &pkg, nil
}

// ListActiveTrafficPackages returns every purchasable package, ordered by
// price ascending. Used both by the admin management page and by resellers
// (web + Telegram bot) picking a package to buy.
func (s *TrafficPackageService) ListActiveTrafficPackages() ([]model.TrafficPackage, error) {
	var pkgs []model.TrafficPackage
	if err := s.db.Where("is_active = ?", true).Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

// ListAllTrafficPackages returns every package regardless of active status,
// for the admin management page (so a deactivated package can still be seen
// and re-activated).
func (s *TrafficPackageService) ListAllTrafficPackages() ([]model.TrafficPackage, error) {
	var pkgs []model.TrafficPackage
	if err := s.db.Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list all traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

func (s *TrafficPackageService) UpdateTrafficPackage(id uint, name *string, description *string, trafficBytes *int64, priceAmount *int64, isActive *bool) (*model.TrafficPackage, error) {
	var pkg model.TrafficPackage
	if err := s.db.First(&pkg, id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch traffic package for update", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		if *name == "" {
			return nil, errors.New("package name is required")
		}
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if trafficBytes != nil {
		if *trafficBytes <= 0 {
			return nil, errors.New("traffic size must be positive")
		}
		updates["traffic_bytes"] = *trafficBytes
	}
	if priceAmount != nil {
		if *priceAmount < 0 {
			return nil, errors.New("price must be non-negative")
		}
		updates["price_amount"] = *priceAmount
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&pkg).Updates(updates).Error; err != nil {
		s.logger.Error("failed to update traffic package", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

// DeleteTrafficPackage soft-deletes the catalog entry. Historical
// PackagePurchase rows are untouched (they store their own denormalized
// snapshot of name/price/size), so past receipts remain accurate even after
// a package is removed from sale.
func (s *TrafficPackageService) DeleteTrafficPackage(id uint) error {
	result := s.db.Delete(&model.TrafficPackage{}, id)
	if result.Error != nil {
		s.logger.Error("failed to delete traffic package", zap.Uint("id", id), zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
