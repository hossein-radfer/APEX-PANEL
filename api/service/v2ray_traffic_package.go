package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// V2RayTrafficPackageService mirrors UserManagerTrafficPackageService
// exactly, but manages the catalog of one-time extra traffic packages for
// the V2Ray quota pool.
type V2RayTrafficPackageService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewV2RayTrafficPackageService(db *gorm.DB) *V2RayTrafficPackageService {
	return &V2RayTrafficPackageService{
		db:     db,
		logger: zap.L().Named("V2RayTrafficPackageService"),
	}
}

func (s *V2RayTrafficPackageService) CreateTrafficPackage(name string, description *string, trafficBytes int64, priceAmount int64) (*model.V2RayTrafficPackage, error) {
	if name == "" {
		return nil, errors.New("package name is required")
	}
	if trafficBytes <= 0 {
		return nil, errors.New("traffic size must be positive")
	}
	if priceAmount < 0 {
		return nil, errors.New("price must be non-negative")
	}

	// Pre-checked here rather than left to Name's DB-level uniqueIndex so
	// the caller gets a clean, actionable error instead of a raw SQLite
	// "UNIQUE constraint failed" string -- a confirmed, reported bug. GORM's
	// default query scope already excludes soft-deleted rows, so a
	// previously deleted package's name is correctly treated as available
	// again (matching what an admin looking at the current, live list
	// would expect) rather than colliding against the still-present
	// soft-deleted row the way the raw DB constraint would.
	var existing model.V2RayTrafficPackage
	err := s.db.Where("name = ?", name).First(&existing).Error
	if err == nil {
		return nil, errors.New("a traffic package with this name already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to check for existing v2ray traffic package", zap.Error(err))
		return nil, err
	}

	pkg := model.V2RayTrafficPackage{
		Name:         name,
		Description:  description,
		TrafficBytes: trafficBytes,
		PriceAmount:  priceAmount,
		IsActive:     true,
	}

	if err := s.db.Create(&pkg).Error; err != nil {
		s.logger.Error("failed to create v2ray traffic package", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

func (s *V2RayTrafficPackageService) GetTrafficPackage(id uint) (*model.V2RayTrafficPackage, error) {
	var pkg model.V2RayTrafficPackage
	if err := s.db.First(&pkg, "id = ? AND is_active = ?", id, true).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch v2ray traffic package", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	return &pkg, nil
}

func (s *V2RayTrafficPackageService) ListActiveTrafficPackages() ([]model.V2RayTrafficPackage, error) {
	var pkgs []model.V2RayTrafficPackage
	if err := s.db.Where("is_active = ?", true).Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list v2ray traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

func (s *V2RayTrafficPackageService) ListAllTrafficPackages() ([]model.V2RayTrafficPackage, error) {
	var pkgs []model.V2RayTrafficPackage
	if err := s.db.Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list all v2ray traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

func (s *V2RayTrafficPackageService) UpdateTrafficPackage(id uint, name *string, description *string, trafficBytes *int64, priceAmount *int64, isActive *bool) (*model.V2RayTrafficPackage, error) {
	var pkg model.V2RayTrafficPackage
	if err := s.db.First(&pkg, id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch v2ray traffic package for update", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		if *name == "" {
			return nil, errors.New("package name is required")
		}
		if *name != pkg.Name {
			var existing model.V2RayTrafficPackage
			err := s.db.Where("name = ? AND id <> ?", *name, id).First(&existing).Error
			if err == nil {
				return nil, errors.New("a traffic package with this name already exists")
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				s.logger.Error("failed to check for existing v2ray traffic package", zap.Error(err))
				return nil, err
			}
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
		s.logger.Error("failed to update v2ray traffic package", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

// DeleteTrafficPackage soft-deletes the catalog entry. Historical
// V2RayPackagePurchase rows are untouched (own denormalized snapshot).
func (s *V2RayTrafficPackageService) DeleteTrafficPackage(id uint) error {
	result := s.db.Delete(&model.V2RayTrafficPackage{}, id)
	if result.Error != nil {
		s.logger.Error("failed to delete v2ray traffic package", zap.Uint("id", id), zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
