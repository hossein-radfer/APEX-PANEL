package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// ErrInterfaceHasIPPool is returned by DeleteInterface when at least one
// IPPool row still references this interface (IPPool.InterfaceID is a
// real foreign key -- see dataservice/model/ip_pool.go -- with
// _pragma=foreign_keys(1) enabled on every sqlite connection). Checked
// explicitly, BEFORE the Mikrotik-side delete below, so a doomed request
// fails cleanly without first deleting the interface from the router and
// only then discovering the local DB delete would violate the
// constraint -- that ordering would leave Mikrotik and the database
// permanently out of sync.
var ErrInterfaceHasIPPool = errors.New("cannot delete interface: one or more IP pools still reference it")

type WgInterface struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewWgInterface(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *WgInterface {
	return &WgInterface{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("WgInterfaceService"),
	}
}

// GetInterfaces returns interfaces. When allowedIDs is non-nil, results are
// restricted to that set (used to scope resellers to their assigned interfaces).
func (i *WgInterface) GetInterfaces(allowedIDs []uint) (*[]schema.InterfaceResponse, error) {
	var interfaces []model.Interface
	query := i.db.Order("created_at desc")
	if allowedIDs != nil {
		if len(allowedIDs) == 0 {
			return &[]schema.InterfaceResponse{}, nil
		}
		query = query.Where("id IN ?", allowedIDs)
	}
	if err := query.Find(&interfaces).Error; err != nil {
		i.logger.Error("failed to get wireguard interfaces from database", zap.Error(err))
		return nil, err
	}

	var wgInterfaces []schema.InterfaceResponse
	for _, iface := range interfaces {
		mtInterface, err := i.mikrotikAdaptor.FetchWgInterface(context.Background(), iface.InterfaceID)
		if err != nil {
			i.logger.Error("failed to fetch wireguard interface from Mikrotik", zap.String("interfaceID", iface.InterfaceID), zap.Error(err))
			return nil, fmt.Errorf("failed to fetch wireguard interface from Mikrotik: %w", err)
		}
		wgInterface := i.transformInterfaceToResponse(iface, mtInterface.MTU, *mtInterface.Running)
		wgInterfaces = append(wgInterfaces, wgInterface)
	}

	return &wgInterfaces, nil
}

func (i *WgInterface) CreateInterface(req *schema.CreateInterfaceRequest) (*schema.InterfaceResponse, error) {
	wgInterface := &mikrotik.WireGuardInterface{
		Name:       req.Name,
		Comment:    req.Comment,
		ListenPort: req.ListenPort,
	}

	mtInterface, err := i.mikrotikAdaptor.CreateWgInterface(context.Background(), *wgInterface)
	if err != nil {
		i.logger.Error("failed to create wireguard interface", zap.Error(err))
		return nil, err
	}

	dbInterface := model.Interface{
		InterfaceID: mtInterface.ID,
		Comment:     wgInterface.Comment,
		Name:        wgInterface.Name,
		PrivateKey:  mtInterface.PrivateKey,
		PublicKey:   mtInterface.PublicKey,
		ListenPort:  wgInterface.ListenPort,
	}

	if err := i.db.Create(&dbInterface).Error; err != nil {
		i.logger.Error("failed to save wireguard interface to database", zap.Error(err))
		return nil, err
	}

	transformedInterface := i.transformInterfaceToResponse(dbInterface, mtInterface.MTU, *mtInterface.Running)
	return &transformedInterface, nil
}

func (i *WgInterface) ToggleInterfaceStatus(id uint) error {
	var iface model.Interface
	if err := i.db.First(&iface, id).Error; err != nil {
		i.logger.Error("failed to find wireguard interface in database", zap.Error(err))
		return fmt.Errorf("failed to find wireguard interface in database: %w", err)
	}

	disabled := strconv.FormatBool(!iface.Disabled)

	wgInterface := mikrotik.WireGuardInterface{
		Disabled: disabled,
	}

	if _, err := i.mikrotikAdaptor.UpdateWgInterface(context.Background(), iface.InterfaceID, wgInterface); err != nil {
		i.logger.Error("failed to update wireguard interface status", zap.Error(err))
		return fmt.Errorf("failed to update wireguard interface status: %w", err)
	}

	if err := i.db.Model(&iface).Update("disabled", disabled).Error; err != nil {
		i.logger.Error("failed to update interface status in database", zap.Error(err))
		return fmt.Errorf("failed to update interface status in database: %w", err)
	}

	return nil
}

func (i *WgInterface) UpdateInterface(id uint, req *schema.UpdateInterfaceRequest) (*schema.InterfaceResponse, error) {
	var iface model.Interface
	if err := i.db.First(&iface, id).Error; err != nil {
		i.logger.Error("failed to get interface from database", zap.Error(err))
		return nil, err
	}

	wgInterface := mikrotik.WireGuardInterface{}

	if req.Disabled != nil {
		disabledStr := strconv.FormatBool(*req.Disabled)
		wgInterface.Disabled = disabledStr
	}
	if req.Comment != nil {
		wgInterface.Comment = req.Comment
	}

	wgInterface.Name = req.Name

	mtInterface, err := i.mikrotikAdaptor.UpdateWgInterface(context.Background(), iface.InterfaceID, wgInterface)
	if err != nil {
		i.logger.Error("failed to update wireguard interface", zap.Error(err))
		return nil, fmt.Errorf("failed to update wireguard interface: %w", err)
	}

	iface.Comment = req.Comment
	iface.Name = wgInterface.Name
	iface.ListenPort = wgInterface.ListenPort

	if err := i.db.Save(&iface).Error; err != nil {
		i.logger.Error("failed to update wireguard interface in database", zap.Error(err))
		return nil, fmt.Errorf("failed to update wireguard interface in database")
	}

	transformedInterface := i.transformInterfaceToResponse(iface, mtInterface.MTU, *mtInterface.Running)
	return &transformedInterface, nil
}

func (i *WgInterface) DeleteInterface(id uint) error {
	var iface model.Interface
	if err := i.db.First(&iface, id).Error; err != nil {
		i.logger.Error("failed to find wireguard interface in database", zap.Error(err))
		return fmt.Errorf("failed to find wireguard interface in database: %w", err)
	}

	var dependentPoolCount int64
	if err := i.db.Model(&model.IPPool{}).Where("interface_id = ?", id).Count(&dependentPoolCount).Error; err != nil {
		i.logger.Error("failed to check for dependent IP pools", zap.Error(err))
		return fmt.Errorf("failed to check for dependent IP pools: %w", err)
	}
	if dependentPoolCount > 0 {
		return ErrInterfaceHasIPPool
	}

	if err := i.mikrotikAdaptor.DeleteWgInterface(context.Background(), iface.InterfaceID); err != nil {
		i.logger.Error("failed to delete wireguard interface from Mikrotik", zap.Error(err))
		return fmt.Errorf("failed to delete wireguard interface from Mikrotik: %w", err)
	}

	if err := i.db.Unscoped().Delete(&iface).Error; err != nil {
		i.logger.Error("failed to delete wireguard interface from database", zap.Error(err))
		return fmt.Errorf("failed to delete wireguard interface from database: %w", err)
	}

	return nil
}

func (i *WgInterface) GetInterfacesData() (*schema.InterfaceStatsResponse, error) {
	var totalInterfaces int64
	if err := i.db.Model(&model.Interface{}).Count(&totalInterfaces).Error; err != nil {
		i.logger.Error("failed to count total interfaces", zap.Error(err))
		return nil, fmt.Errorf("failed to count total interfaces: %w", err)
	}

	var activeInterfaces int64
	if err := i.db.Model(&model.Interface{}).Where("disabled = ?", false).Count(&activeInterfaces).Error; err != nil {
		i.logger.Error("failed to count active interfaces", zap.Error(err))
		return nil, fmt.Errorf("failed to count active interfaces: %w", err)
	}

	return &schema.InterfaceStatsResponse{
		TotalInterfaces:  int(totalInterfaces),
		ActiveInterfaces: int(activeInterfaces),
	}, nil
}

func (i *WgInterface) transformInterfaceToResponse(wgInterface model.Interface, mtu, status string) schema.InterfaceResponse {
	return schema.InterfaceResponse{
		Id:          wgInterface.ID,
		InterfaceID: wgInterface.InterfaceID,
		Disabled:    wgInterface.Disabled,
		Comment:     wgInterface.Comment,
		Name:        wgInterface.Name,
		ListenPort:  wgInterface.ListenPort,
		MTU:         mtu,
		IsRunning:   status == "true",
	}
}
