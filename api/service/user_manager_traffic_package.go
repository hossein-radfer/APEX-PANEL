package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// UserManagerTrafficPackageService mirrors TrafficPackageService exactly,
// but manages the catalog of one-time extra traffic packages for the User
// Manager (L2TP/PPTP/SSTP/OpenVPN) quota pool.
type UserManagerTrafficPackageService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUserManagerTrafficPackageService(db *gorm.DB) *UserManagerTrafficPackageService {
	return &UserManagerTrafficPackageService{
		db:     db,
		logger: zap.L().Named("UserManagerTrafficPackageService"),
	}
}

func (s *UserManagerTrafficPackageService) CreateTrafficPackage(name string, description *string, trafficBytes int64, priceAmount int64) (*model.UserManagerTrafficPackage, error) {
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
	// "UNIQUE constraint failed" string (see the identical, confirmed bug
	// fixed in V2RayTrafficPackageService.CreateTrafficPackage). GORM's
	// default query scope already excludes soft-deleted rows, so a
	// previously deleted package's name is correctly treated as available
	// again.
	var existing model.UserManagerTrafficPackage
	err := s.db.Where("name = ?", name).First(&existing).Error
	if err == nil {
		return nil, errors.New("a traffic package with this name already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to check for existing user manager traffic package", zap.Error(err))
		return nil, err
	}

	pkg := model.UserManagerTrafficPackage{
		Name:         name,
		Description:  description,
		TrafficBytes: trafficBytes,
		PriceAmount:  priceAmount,
		IsActive:     true,
	}

	if err := s.db.Create(&pkg).Error; err != nil {
		s.logger.Error("failed to create user manager traffic package", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

func (s *UserManagerTrafficPackageService) GetTrafficPackage(id uint) (*model.UserManagerTrafficPackage, error) {
	var pkg model.UserManagerTrafficPackage
	if err := s.db.First(&pkg, "id = ? AND is_active = ?", id, true).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch user manager traffic package", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	return &pkg, nil
}

func (s *UserManagerTrafficPackageService) ListActiveTrafficPackages() ([]model.UserManagerTrafficPackage, error) {
	var pkgs []model.UserManagerTrafficPackage
	if err := s.db.Where("is_active = ?", true).Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list user manager traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

func (s *UserManagerTrafficPackageService) ListAllTrafficPackages() ([]model.UserManagerTrafficPackage, error) {
	var pkgs []model.UserManagerTrafficPackage
	if err := s.db.Order("price_amount ASC").Find(&pkgs).Error; err != nil {
		s.logger.Error("failed to list all user manager traffic packages", zap.Error(err))
		return nil, err
	}

	return pkgs, nil
}

func (s *UserManagerTrafficPackageService) UpdateTrafficPackage(id uint, name *string, description *string, trafficBytes *int64, priceAmount *int64, isActive *bool) (*model.UserManagerTrafficPackage, error) {
	var pkg model.UserManagerTrafficPackage
	if err := s.db.First(&pkg, id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch user manager traffic package for update", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		if *name == "" {
			return nil, errors.New("package name is required")
		}
		if *name != pkg.Name {
			var existing model.UserManagerTrafficPackage
			err := s.db.Where("name = ? AND id <> ?", *name, id).First(&existing).Error
			if err == nil {
				return nil, errors.New("a traffic package with this name already exists")
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				s.logger.Error("failed to check for existing user manager traffic package", zap.Error(err))
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
		s.logger.Error("failed to update user manager traffic package", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}

	return &pkg, nil
}

// DeleteTrafficPackage soft-deletes the catalog entry. Historical
// UserManagerPackagePurchase rows are untouched (own denormalized snapshot).
func (s *UserManagerTrafficPackageService) DeleteTrafficPackage(id uint) error {
	result := s.db.Delete(&model.UserManagerTrafficPackage{}, id)
	if result.Error != nil {
		s.logger.Error("failed to delete user manager traffic package", zap.Uint("id", id), zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
