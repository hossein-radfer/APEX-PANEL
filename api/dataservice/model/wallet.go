package model

import "time"

// Wallet represents a reseller's financial account. This panel bills
// exclusively in Toman (Iran's currency, no meaningful sub-unit in
// practical use), so BalanceAmount is a plain whole-Toman integer -- no
// cents/sub-unit scaling, no per-wallet Currency field, matching the
// AmountToman convention established in model/accounting.go.
type Wallet struct {
	Model
	ResellerID    uint      `gorm:"uniqueIndex;not null;foreignKey:ResellerID;references:ID"`
	BalanceAmount int64     `gorm:"type:bigint;not null;default:0"` // whole Toman
	IsFrozen      bool      `gorm:"not null;default:false"`
	FrozenReason  *string   `gorm:"type:text"`
	LastUsedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// LedgerEntry represents a single financial transaction. Amount/BalanceAfter
// are plain whole-Toman integers -- see Wallet's doc comment for the
// rationale.
type LedgerEntry struct {
	Model
	ResellerID     uint   `gorm:"index;not null;foreignKey:ResellerID;references:ID"`
	WalletID       uint   `gorm:"index;not null;foreignKey:WalletID;references:ID"`
	TransactionID  string `gorm:"type:varchar(128);uniqueIndex;not null"` // idempotency key
	EntryType      string `gorm:"type:varchar(32);not null"`              // CREDIT, DEBIT, ADJUSTMENT
	Amount         int64  `gorm:"type:bigint;not null"`                   // whole Toman, positive for credit, negative for debit
	BalanceAfter   int64  `gorm:"type:bigint;not null"`                   // whole Toman, balance after this transaction
	Description    string `gorm:"type:text"`
	ReferenceType  *string `gorm:"type:varchar(32)"` // PEER_CREATE, PEER_RENEWAL, RECHARGE, REFUND, ADJUSTMENT
	ReferenceID    *uint   `gorm:"index"`             // ID of referenced entity (peer, invoice, etc)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Invoice represents a billing document. Amount is a plain whole-Toman
// integer -- see Wallet's doc comment for the rationale.
type Invoice struct {
	Model
	ResellerID       uint    `gorm:"index;not null;foreignKey:ResellerID;references:ID"`
	InvoiceNumber    string  `gorm:"type:varchar(64);uniqueIndex;not null"`
	Status           string  `gorm:"type:varchar(32);not null;default:'DRAFT'"` // DRAFT, ISSUED, PAID, CANCELLED
	Amount           int64   `gorm:"type:bigint;not null"`                      // whole Toman
	Description      string  `gorm:"type:text"`
	PeriodStart      time.Time
	PeriodEnd        time.Time
	IssuedAt         *time.Time
	DueDate          *time.Time
	PaidAt           *time.Time
	Notes            *string `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PricePlan defines a service plan with pricing. BasePriceAmount is a plain
// whole-Toman integer -- see Wallet's doc comment for the rationale.
type PricePlan struct {
	Model
	Name            string `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description     *string `gorm:"type:text"`
	BasePriceAmount int64  `gorm:"type:bigint;not null"` // whole Toman, price for base service
	BillingInterval string `gorm:"type:varchar(32);not null;default:'MONTHLY'"` // MONTHLY, QUARTERLY, ANNUAL, PAY_AS_YOU_GO
	TrafficAllowance *int64 `gorm:"type:bigint"`                                 // in bytes, nil = unlimited
	MaxPeers        *int32 `gorm:"type:int"`
	MaxServers      *int32 `gorm:"type:int"`
	IsActive        bool   `gorm:"not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
