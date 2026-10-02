package service

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

const (
	AuditActionPeerCreated    = "PEER_CREATED"
	AuditActionPeerDeleted    = "PEER_DELETED"
	AuditActionWalletCredited = "WALLET_CREDITED"
	AuditActionWalletDebited  = "WALLET_DEBITED"
	AuditActionProfileUpdated = "PROFILE_UPDATED"
	AuditActionInvoicePaid    = "INVOICE_PAID"

	AuditActionUserManagerAccountCreated = "USER_MANAGER_ACCOUNT_CREATED"
	AuditActionUserManagerAccountDeleted = "USER_MANAGER_ACCOUNT_DELETED"

	AuditActionV2RayPackageCreated = "V2RAY_PACKAGE_CREATED"
	AuditActionV2RayPackageDeleted = "V2RAY_PACKAGE_DELETED"

	AuditActionApplicationCreated = "APPLICATION_CREATED"
	AuditActionApplicationDeleted = "APPLICATION_DELETED"

	AuditActionDNSAccountCreated = "DNS_ACCOUNT_CREATED"
	AuditActionDNSAccountDeleted = "DNS_ACCOUNT_DELETED"
)

type AuditLog struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewAuditLog(db *gorm.DB) *AuditLog {
	return &AuditLog{
		db:     db,
		logger: zap.L().Named("AuditLogService"),
	}
}

// Log records a reseller action. Failures are logged but never propagated —
// auditing must never block or fail the operation it is observing.
func (a *AuditLog) Log(resellerID uint, resellerName, action, description string) {
	entry := model.AuditLog{
		ResellerID:   resellerID,
		ResellerName: resellerName,
		Action:       action,
		Description:  description,
	}
	if err := a.db.Create(&entry).Error; err != nil {
		a.logger.Error("failed to write audit log entry",
			zap.Uint("resellerID", resellerID),
			zap.String("action", action),
			zap.Error(err),
		)
	}
}

// ListRecent returns the most recent audit log entries across all resellers,
// newest first, for the admin dashboard's activity feed.
func (a *AuditLog) ListRecent(limit int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var entries []model.AuditLog
	if err := a.db.Order("created_at DESC").Limit(limit).Find(&entries).Error; err != nil {
		a.logger.Error("failed to list audit log entries", zap.Error(err))
		return nil, err
	}
	return entries, nil
}
