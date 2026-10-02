package model

import "time"

// V2RayPackage is one customer's V2Ray service -- a fully independent VPN
// product from Peer (WireGuard) and UserManagerAccount (L2TP/PPTP/SSTP/
// OpenVPN), deliberately not sharing a table with either. Unlike those two,
// a single package fans out to a client on EVERY registered XuiPanel (see
// V2RayPackageLocation) -- usage is the sum across all of a package's
// locations, and CombinedConfigCached is the pre-merged subscription blob
// covering every location, so the public share endpoint never needs to
// contact any x-ui server at request time (see V2RayPackageLocation's doc
// comment for where that sync actually happens).
type V2RayPackage struct {
	Model
	UUID string `gorm:"type:varchar(36);uniqueIndex;not null"` // for the public share link, mirrors Peer.UUID/UserManagerAccount.UUID

	// CustomerLabel is free-text -- this codebase has no separate customer
	// table for any VPN product, so this mirrors how Peer/UserManagerAccount
	// identify their owner (a Comment-style label), not a foreign key.
	CustomerLabel *string `gorm:"type:varchar(255)"`

	// Comment is a separate, optional, admin-only free-text note --
	// distinct from CustomerLabel (which IS customer-facing: it appears in
	// the subscription title, see buildInfoConfigLine/rewriteV2RaySubscriptionTitle).
	// Comment is NEVER read by anything that builds subscription content or
	// a Telegram bot message; it exists purely for the admin's own internal
	// bookkeeping on the package list/edit UI, matching Peer.Comment's own
	// identical "admin-only note, never customer-facing" role.
	Comment *string `gorm:"type:text"`

	// ResellerID nil (IS NULL) means an admin-direct package, matching
	// UserManagerAccount's convention -- NOT Peer's "nil = no filter"
	// convention. See V2RayPackageService's scoping methods.
	ResellerID *uint `gorm:"index"`

	TotalVolumeBytes int64 `gorm:"type:bigint;not null"`
	DurationDays     int   `gorm:"type:int;not null"`
	StartAt          *time.Time
	ExpireAt         *time.Time

	Status string `gorm:"type:varchar(16);not null;default:'active'"` // "active" | "suspended" | "expired"

	// SuspendedByQuota/WasActiveBeforeSuspend mirror Peer/UserManagerAccount's
	// quota-suspend bookkeeping exactly, scoped to THIS package's own
	// TotalVolumeBytes limit only (see V2RaySyncService.enforcePackageQuota).
	SuspendedByQuota       bool `gorm:"type:boolean;not null;default:false"`
	WasActiveBeforeSuspend bool `gorm:"type:boolean;not null;default:false"`

	// SuspendedByResellerQuota tracks whether THIS package was disabled
	// because the OWNING RESELLER's overall V2Ray quota (Reseller.
	// V2RayQuotaBytes/V2RayUsedBytes, summed across every one of that
	// reseller's packages) was exceeded -- a confirmed, reported incident:
	// a reseller with an 800GB overall V2Ray quota was found to have
	// actually used over 1000GB, because applyResellerV2RayQuota
	// previously only ever UPDATED the reseller's usage figure and never
	// enforced it -- each package was independently checked only against
	// its OWN small TotalVolumeBytes limit (SuspendedByQuota above), so a
	// reseller with many small packages could freely exceed their overall
	// quota as long as no single package individually went over its own
	// limit. Deliberately a SEPARATE flag from SuspendedByQuota -- that
	// one is this package's OWN limit, a completely different concept
	// from the reseller-wide pool; conflating the two (as WireGuard/
	// UserManager's own SuspendedByQuota once wrongly did for their
	// per-resource TrafficLimit, see Peer.SuspendedByTrafficLimit's own
	// doc comment for that earlier, separately-fixed bug) would make
	// resuming one either incorrectly resume/block the other.
	SuspendedByResellerQuota bool `gorm:"type:boolean;not null;default:false"`

	// UsageOffsetBytes implements the admin's own "Reset Usage" action for
	// this package -- mirrors model.UserManagerAccount.UsageOffsetDownload/
	// UsageOffsetUpload's exact same reasoning, adapted for V2Ray: each
	// V2RayPackageLocation.UsedBytesCached is a delta-ACCUMULATED running
	// total this panel itself maintains (see V2RaySyncService.
	// syncOneLocation), and Reseller.V2RayUsedBytes is a live SUM of every
	// location's raw UsedBytesCached across that reseller (see
	// applyResellerV2RayQuota) -- so zeroing UsedBytesCached directly would
	// immediately shrink the reseller's quota total too. Instead,
	// DisplayedUsedBytes = SUM(UsedBytesCached) - UsageOffsetBytes (clamped
	// at 0): "reset" bumps the offset up to the package's current raw sum,
	// which both transformPackageToResponse's own display AND
	// enforcePackageQuota's overQuota check read from, while
	// UsedBytesCached itself (and therefore the reseller pool) is never
	// touched. See V2RayPackageService.ResetUsage.
	UsageOffsetBytes int64 `gorm:"type:bigint;not null;default:0"`

	// Share/connection-info page, mirroring Peer.IsShared/ShareExpireTime.
	IsShared        bool    `gorm:"type:boolean;not null;default:false"`
	ShareExpireTime *string `gorm:"type:varchar(255)"`

	// CombinedConfigCached is the ONLY thing the public share/subscription
	// endpoint reads -- a base64 blob combining every location's
	// ConfigLinkCached, written exclusively by the background sync job
	// (service.SyncPackageUsage), never computed at request time. Empty
	// until the first sync tick after creation.
	CombinedConfigCached   string `gorm:"type:text;not null;default:''"`
	CombinedConfigSyncedAt *time.Time
}
