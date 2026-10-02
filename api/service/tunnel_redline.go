package service

import (
	"strings"

	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TunnelRedlineValidator is spec section ب-6's "منطقه امن مدیریتی" gate --
// a list of interfaces/IPs/ports the admin uses for their OWN management
// connection to a server. This is DELIBERATELY its own type in its own
// file, with its own DB read, called from the remediation engine but
// never sharing state or logic with it -- the spec's explicit
// requirement: "این چک باید مستقل از منطق تصمیم‌گیری باشد" (this check
// must be independent of the decision logic), so a bug in
// TunnelHealthService's decision-making can never accidentally bypass
// it. Every mutating action MUST call IsProtected before being built,
// let alone sent -- if it returns true, the caller must refuse to even
// construct the command, not just skip sending it.
type TunnelRedlineValidator struct {
	db *gorm.DB
}

func NewTunnelRedlineValidator(db *gorm.DB) *TunnelRedlineValidator {
	return &TunnelRedlineValidator{db: db}
}

// IsProtectedInterface reports whether name matches a redline entry of
// kind "interface" -- checked before any interface-toggle action.
func (v *TunnelRedlineValidator) IsProtectedInterface(name string) bool {
	return v.matches("interface", name)
}

// IsProtectedIP reports whether ip matches a redline entry of kind "ip".
func (v *TunnelRedlineValidator) IsProtectedIP(ip string) bool {
	return v.matches("ip", ip)
}

// IsProtectedPort reports whether port matches a redline entry of kind
// "port".
func (v *TunnelRedlineValidator) IsProtectedPort(port string) bool {
	return v.matches("port", port)
}

func (v *TunnelRedlineValidator) matches(kind, value string) bool {
	if value == "" {
		return false
	}
	var entries []model.ManagementRedlineEntry
	if err := v.db.Where("kind = ?", kind).Find(&entries).Error; err != nil {
		// Fail CLOSED: if the redline list can't be read, treat every
		// target as protected rather than silently allowing a mutating
		// action through -- a DB error here must never become a bypass.
		return true
	}
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Value), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

// ListEntries returns every configured redline entry -- the "منطقه امن
// مدیریتی" subpage's own read endpoint.
func (v *TunnelRedlineValidator) ListEntries() ([]model.ManagementRedlineEntry, error) {
	var entries []model.ManagementRedlineEntry
	if err := v.db.Order("kind asc, value asc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (v *TunnelRedlineValidator) AddEntry(kind, value string, comment *string) (*model.ManagementRedlineEntry, error) {
	entry := model.ManagementRedlineEntry{Kind: kind, Value: value, Comment: comment}
	if err := v.db.Create(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (v *TunnelRedlineValidator) RemoveEntry(id uint) error {
	return v.db.Delete(&model.ManagementRedlineEntry{}, id).Error
}
