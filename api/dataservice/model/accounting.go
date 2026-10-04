package model

import "time"

// This file implements the admin's own private profit/loss bookkeeping --
// a fully separate concern from Wallet/LedgerEntry/Invoice/PricePlan
// (those track a RESELLER's prepaid credit with THIS panel; nothing here
// touches that system or its tables). AccountingPartner/AccountingCost
// track what the admin PAYS OUT (server rent, panel access, any other
// recurring or one-time supplier cost); AccountingCustomerPayment tracks
// what the admin RECEIVES from their own end customers, which this panel
// otherwise has no concept of at all (a Peer/UserManagerAccount/
// V2RayPackage's "customer" is just a free-text label, never a billing
// relationship). Profit is derived (SUM(payments) - SUM(costs) over a
// period), not stored -- see AccountingService.GetSummary.

// AccountingPartner is a supplier/partner the admin pays money to -- a
// server provider, a panel reseller, anyone on the COST side of the
// ledger. Free-text Name, matching how Peer.Comment/V2RayPackage.
// CustomerLabel already identify a party by label rather than a foreign
// key elsewhere in this codebase (there being no shared "contact" table).
type AccountingPartner struct {
	Model
	Name    string  `gorm:"type:varchar(255);not null"`
	Comment *string `gorm:"type:text"`
}

// AccountingCost is one payment the admin made (or owes) to a partner --
// optionally tied to a specific model.Server and/or a protocol label
// ("wireguard"/"v2ray"/"user_manager"/nil for a general cost not specific
// to one protocol), matching the admin's own explicit "cost tracking per
// protocol/server" requirement. AmountToman is a plain integer -- Toman has
// no subunit in everyday use, and this is also how Wallet/Invoice/
// PricePlan/TrafficPackage's own amount fields are stored, since this
// panel bills exclusively in Toman.
//
// RecurrenceIntervalDays + NextDueAt implement "recurring server-renewal
// invoices": nil RecurrenceIntervalDays means a one-time cost; a non-nil
// value means AccountingScheduler (cmd/jobs) advances NextDueAt by that
// many days and creates a fresh reminder each time it comes due, rather
// than this row itself repeating -- mirrors BotSettings.AutoBackupHour/
// AutoReportHour's own "a schedule config the job reads, not a
// self-rescheduling row" pattern.
type AccountingCost struct {
	Model
	PartnerID uint    `gorm:"index;not null"`
	ServerID  *uint   `gorm:"index"`
	Protocol  *string `gorm:"type:varchar(32)"` // "wireguard" | "user_manager" | "v2ray" | nil
	// ManualServerName (confirmed, reported gap): a free-text server
	// label for costs that belong to infrastructure NOT present in the
	// Server table -- that table only ever holds RouterOS routers this
	// panel actively manages via its API, so a foreign VPS/dedicated
	// server (e.g. an x-ui/DNS host abroad with no RouterOS API at all)
	// could never be selected as ServerID, even though nothing about
	// ServerID itself is Iran-only or otherwise geographically
	// restricted. Mutually exclusive with ServerID in the UI (the admin
	// either picks a managed router or types a label, never both) but
	// both are nullable at the DB level so existing rows are unaffected.
	ManualServerName *string `gorm:"type:varchar(255)"`
	// LocationKey identifies the SAME location a sale's LocationKey does
	// (Interface.ID/Group name/XuiPanel.ID, formatted as a string) -- see
	// AccountingCustomerPayment.LocationKey's own doc comment. ServerID
	// above is a coarser, optional link to the whole RouterOS/x-ui HOST a
	// cost is for (e.g. "this $8/mo VPS rent"); LocationKey is the finer
	// per-interface/group/panel key needed to roll a location's costs up
	// against that SAME location's sales (AccountingService.
	// GetLocationProfitability) -- a Server has no direct link to the
	// Interfaces that live on it in this codebase, so ServerID alone
	// can't answer "what did THIS interface cost."
	LocationKey *string `gorm:"type:varchar(64);index"`
	AmountToman int64   `gorm:"type:bigint;not null"`
	Description *string `gorm:"type:text"`
	PaidAt      *time.Time
	DueAt       *time.Time
	// RecurrenceIntervalDays/NextDueAt: see doc comment above. Both nil
	// together means a plain one-time cost.
	RecurrenceIntervalDays *int       `gorm:"type:int"`
	NextDueAt              *time.Time `gorm:"index"`
	// ReminderSentAt latches the last time AccountingScheduler notified the
	// admin this recurring cost is due, mirroring Reseller.QuotaWarningSent's
	// own "send once per period, not once per tick" convention -- cleared
	// back to nil every time NextDueAt advances.
	ReminderSentAt *time.Time
}

// AccountingSaleProtocol values for AccountingCustomerPayment.Protocol --
// deliberately the exact same three strings AccountingCost.Protocol uses,
// so a sale and a cost can be joined/grouped by protocol consistently.
const (
	AccountingSaleWireGuard   = "wireguard"
	AccountingSaleUserManager = "user_manager"
	AccountingSaleV2Ray       = "v2ray"
)

// AccountingCustomerPayment is one sale to an end customer -- the admin's
// own explicit requirement: record what a specific WireGuard peer, User
// Manager account, or V2Ray package actually SOLD for (AmountToman) and
// what it COST the admin to provision (CostToman), so per-user profit
// (AmountToman - CostToman) and per-location profit (SUM(AmountToman) -
// SUM(AccountingCost.AmountToman) for that location) are both directly
// computable -- see AccountingService.GetUserProfitability/
// GetLocationProfitability.
//
// Protocol + ResourceID identify WHICH resource was sold, mirroring
// ApplicationResourceLocation's exact polymorphic-by-protocol convention
// (see that model's own doc comment) rather than three separate nullable
// foreign keys: ResourceID is the Peer.ID/UserManagerAccount.ID/
// V2RayPackage.ID depending on Protocol. Both are optional -- a payment
// logged before this feature existed, or an ad-hoc/bulk payment not tied
// to one specific account, still has CustomerLabel as its free-text
// identity exactly as before.
//
// LocationKey is the resource's LOCATION at the moment of sale (the
// Interface.ID/Group name/XuiPanel.ID formatted as a string, matching
// ApplicationResourceLocation.ResourceKey's own string-for-both-numeric-
// and-name-keys convention) -- denormalized onto the sale row (not
// re-derived from the live Peer/Account/Package each time) so a location's
// historical revenue stays correct even after the underlying resource is
// later moved to a different interface/group/panel or deleted entirely.
//
// ReceiptFilePath is optional -- see AccountingReceiptFile's own doc
// comment for where the physical file lives.
type AccountingCustomerPayment struct {
	Model
	CustomerLabel   string    `gorm:"type:varchar(255);not null"`
	AmountToman     int64     `gorm:"type:bigint;not null"`
	CostToman       int64     `gorm:"type:bigint;not null;default:0"`
	Protocol        *string   `gorm:"type:varchar(32);index"`
	ResourceID      *uint     `gorm:"index"`
	LocationKey     *string   `gorm:"type:varchar(64);index"`
	Note            *string   `gorm:"type:text"`
	ReceiptFilePath *string   `gorm:"type:varchar(255)"`
	PaidAt          time.Time `gorm:"not null;index"`
}
