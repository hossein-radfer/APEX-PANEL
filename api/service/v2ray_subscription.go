package service

import (
	"encoding/base64"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// rewriteV2RaySubscriptionTitle decodes panel's raw subscription content
// for one location's package -- a newline-separated list of vless/vmess/
// ... links, either base64-encoded (x-ui's "Subscription Encode" setting
// on, the default) OR plaintext (that same setting turned off, in which
// case /sub/{subId} returns the raw link text directly -- see
// isRawLinkContent) -- replaces every link's title (the part after '#')
// with the resolved sale title for packageID, and returns PLAINTEXT (not
// re-encoded). Package-level (not a method on either V2RaySyncService or
// V2RayPackageService) specifically so BOTH can call it: the periodic
// sync job (V2RaySyncService.syncOneLocation) on every tick, AND package
// creation (V2RayPackageService.CreatePackage) once immediately after a
// successful AddClient, so a brand-new package's config links/QR are
// populated right away instead of sitting empty until the next scheduled
// tick -- a confirmed, reported bug: a customer handed a freshly-created
// package's share link to their own customer and it showed blank config
// links/QR codes until the next cron tick ran, which reads as "this panel
// is broken" to someone who doesn't know a background job exists at all.
//
// See V2RayPackageLocation.ConfigLinkCached's own doc comment for why the
// value stored/returned here is plaintext, not base64 (only the FINAL
// cross-panel-combined result, computed by recombineConfig, is base64
// encoded, exactly once).
func rewriteV2RaySubscriptionTitle(db *gorm.DB, rawSub string, packageID uint, panel model.XuiPanel) (string, error) {
	trimmed := strings.TrimSpace(rawSub)

	var decoded []byte
	if isRawLinkContent(trimmed) {
		// This panel has its own "Subscription Encode" setting turned OFF,
		// so /sub/{subId} returns the plain vless://... link text directly,
		// NOT base64 -- see isRawLinkContent's own doc comment for the full
		// rationale (a confirmed, reported bug this branch fixes).
		decoded = []byte(trimmed)
	} else {
		var err error
		decoded, err = base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			return "", fmt.Errorf("subscription content is neither a raw link nor valid base64: %w", err)
		}
	}

	var pkg model.V2RayPackage
	if err := db.Select("reseller_id").First(&pkg, packageID).Error; err != nil {
		return "", fmt.Errorf("package not found: %w", err)
	}

	title := resolveV2RaySaleTitle(db, pkg.ResellerID, panel)

	lines := strings.Split(strings.TrimSpace(string(decoded)), "\n")
	rewritten := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if hashIdx := strings.LastIndex(line, "#"); hashIdx != -1 {
			line = line[:hashIdx+1] + urlEncodeTitle(title)
		}
		rewritten = append(rewritten, line)
	}

	return strings.Join(rewritten, "\n"), nil
}

// resolveV2RaySaleTitle is the single shared implementation of the
// custom-title-falling-back-to-panel-default rule, used by both
// V2RayPackageService.resolveSaleTitle (for the share page / admin UI)
// and rewriteV2RaySubscriptionTitle above (for the actual link text) --
// previously duplicated as two separate near-identical methods on the two
// services, which is exactly the kind of drift risk that made this
// extraction worthwhile once a THIRD call site (package creation) needed
// the same logic.
func resolveV2RaySaleTitle(db *gorm.DB, resellerID *uint, panel model.XuiPanel) string {
	if resellerID == nil {
		return panel.SaleTitle
	}

	var custom model.ResellerV2RaySaleTitle
	if err := db.Where("reseller_id = ? AND panel_id = ?", *resellerID, panel.ID).First(&custom).Error; err != nil {
		return panel.SaleTitle
	}
	return custom.Title
}
