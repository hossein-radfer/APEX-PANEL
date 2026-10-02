package model

// ResellerUserManagerGroup restricts which RouterOS User Manager groups a
// reseller may assign when creating an account -- mirrors ResellerInterface
// exactly (a join table checked with "must be explicitly assigned", no
// empty-means-all fallback), so a reseller with CanCreateUserManagerAccounts
// but zero assigned groups simply cannot create an account until the admin
// assigns at least one, same as an unassigned-interface reseller can't
// create a WireGuard peer.
type ResellerUserManagerGroup struct {
	Model
	ResellerID uint   `gorm:"uniqueIndex:idx_reseller_um_group;not null"`
	GroupName  string `gorm:"type:varchar(255);uniqueIndex:idx_reseller_um_group;not null"`
}

// ResellerUserManagerProfile mirrors ResellerUserManagerGroup exactly, for
// RouterOS User Manager profiles instead of groups.
type ResellerUserManagerProfile struct {
	Model
	ResellerID  uint   `gorm:"uniqueIndex:idx_reseller_um_profile;not null"`
	ProfileName string `gorm:"type:varchar(255);uniqueIndex:idx_reseller_um_profile;not null"`
}
