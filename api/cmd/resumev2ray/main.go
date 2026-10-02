// Command resumev2ray is a one-off operational tool for manually
// triggering V2Ray package resume/repair logic outside the normal HTTP
// update path, for cases where a package or reseller's packages are
// stuck suspended despite the underlying condition being resolved.
// Supports three modes: repairing locations left disabled on the x-ui
// panel after a client was deleted there directly, resuming packages for
// a reseller whose quota was raised without the automatic resume path
// having run, and clearing a stale billing-suspended state. Reuses the
// same production code paths (DB connection, XuiPanelService,
// V2RaySyncService) rather than a reimplementation. Not meant to be left
// installed long-term -- a one-shot operational script.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/service"
)

func usage() {
	fmt.Println("usage:")
	fmt.Println("  resumev2ray repair <package_id> [package_id...]")
	fmt.Println("  resumev2ray resume <reseller_id>")
	fmt.Println("  resumev2ray resume-billing <reseller_id>")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		usage()
	}

	db, err := dataservice.ConnectDB(config.GetDBConfig())
	if err != nil {
		fmt.Println("failed to connect to db:", err)
		os.Exit(1)
	}

	panels := service.NewXuiPanelService(db)
	sync := service.NewV2RaySyncService(db, panels)

	switch os.Args[1] {
	case "repair":
		for _, arg := range os.Args[2:] {
			packageID, err := strconv.ParseUint(arg, 10, 64)
			if err != nil {
				fmt.Println("invalid package id:", arg, err)
				continue
			}
			fmt.Printf("repairing disabled locations for package %d...\n", packageID)
			if err := sync.RepairDisabledLocationsForPackage(uint(packageID)); err != nil {
				fmt.Println("error:", err)
				continue
			}
			fmt.Println("done")
		}
	case "resume":
		resellerID, err := strconv.ParseUint(os.Args[2], 10, 64)
		if err != nil {
			fmt.Println("invalid reseller id:", os.Args[2], err)
			os.Exit(1)
		}
		fmt.Printf("resuming reseller-quota-suspended v2ray packages for reseller %d...\n", resellerID)
		sync.ResumePackagesForResellerQuota(uint(resellerID))
		fmt.Println("done")
	case "resume-billing":
		resellerID, err := strconv.ParseUint(os.Args[2], 10, 64)
		if err != nil {
			fmt.Println("invalid reseller id:", os.Args[2], err)
			os.Exit(1)
		}
		fmt.Printf("clearing billing_suspended for reseller %d...\n", resellerID)
		if err := db.Model(&model.Reseller{}).Where("id = ?", uint(resellerID)).Update("billing_suspended", false).Error; err != nil {
			fmt.Println("failed to clear billing_suspended:", err)
			os.Exit(1)
		}
		fmt.Printf("resuming billing-suspended v2ray packages for reseller %d...\n", resellerID)
		sync.ResumePackagesForReseller(uint(resellerID))
		fmt.Println("done")
	default:
		usage()
	}
}
