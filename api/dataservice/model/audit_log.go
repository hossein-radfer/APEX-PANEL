package model

// AuditLog records a notable action taken by a reseller, so admins can see
// what their resellers have been doing (peer creation, wallet changes,
// credential changes) without digging through raw request logs.
type AuditLog struct {
	Model
	ResellerID   uint   `gorm:"index;not null"`
	ResellerName string `gorm:"type:varchar(255);not null"` // denormalized snapshot at log time, survives rename/deletion
	Action       string `gorm:"type:varchar(64);not null"`  // e.g. PEER_CREATED, WALLET_CREDITED, PROFILE_UPDATED
	Description  string `gorm:"type:text;not null"`
}
