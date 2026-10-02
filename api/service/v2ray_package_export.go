package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
)

// v2rayExportRow is one package's data for the bulk-create export -- built
// once per package, then rendered into either an .xlsx sheet or a .txt
// file depending on the caller's requested format.
type v2rayExportRow struct {
	CustomerLabel    string
	ShareLink        string
	SubscriptionLink string
}

// ExportPackages builds an .xlsx or .txt file listing req.PackageIDs (every
// package individually access-checked via getPackageByIDScoped, so a
// reseller can never export a package that isn't theirs even if they guess
// another package's ID), with ShareLink/SubscriptionLink columns included
// per req.IncludeShareLink/IncludeSubscriptionLink -- the admin's own
// explicit "دوتا تیک... هم بتونم تکی انتخاب کنم هم دوتاش" requirement.
// Returns the path to a freshly-written temp file; the caller (the HTTP
// handler) is responsible for streaming it back and cleaning it up
// afterward, mirroring BackupService.CreateBackup's own
// (path, cleanup func, error) contract.
func (s *V2RayPackageService) ExportPackages(req *schema.ExportV2RayPackagesRequest, resellerID *uint) (filePath string, cleanup func(), err error) {
	rows := make([]v2rayExportRow, 0, len(req.PackageIDs))
	for _, id := range req.PackageIDs {
		pkg, err := s.getPackageByIDScoped(id, resellerID)
		if err != nil {
			return "", nil, fmt.Errorf("package %d not found or not accessible: %w", id, err)
		}

		label := fmt.Sprintf("package-%d", pkg.ID)
		if pkg.CustomerLabel != nil && *pkg.CustomerLabel != "" {
			label = *pkg.CustomerLabel
		}

		row := v2rayExportRow{CustomerLabel: label}
		baseURL := strings.TrimSuffix(req.BaseURL, "/")
		if req.IncludeShareLink {
			row.ShareLink = fmt.Sprintf("%s/v2ray-share?shareId=%s", baseURL, pkg.UUID)
		}
		if req.IncludeSubscriptionLink {
			row.SubscriptionLink = fmt.Sprintf("%s/api/v2ray-sub/%s", baseURL, pkg.UUID)
		}
		rows = append(rows, row)
	}

	if req.Format == "txt" {
		return s.exportPackagesAsTxt(rows)
	}
	return s.exportPackagesAsXlsx(rows)
}

func (s *V2RayPackageService) exportPackagesAsTxt(rows []v2rayExportRow) (string, func(), error) {
	var sb strings.Builder
	for _, row := range rows {
		sb.WriteString(row.CustomerLabel)
		if row.ShareLink != "" {
			sb.WriteString("\n  Share:        " + row.ShareLink)
		}
		if row.SubscriptionLink != "" {
			sb.WriteString("\n  Subscription: " + row.SubscriptionLink)
		}
		sb.WriteString("\n\n")
	}

	f, err := os.CreateTemp("", "v2ray-packages-*.txt")
	if err != nil {
		s.logger.Error("failed to create temp file for v2ray package txt export", zap.Error(err))
		return "", nil, err
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		s.logger.Error("failed to write v2ray package txt export", zap.Error(err))
		os.Remove(f.Name())
		return "", nil, err
	}

	path := f.Name()
	return path, func() { os.Remove(path) }, nil
}

func (s *V2RayPackageService) exportPackagesAsXlsx(rows []v2rayExportRow) (string, func(), error) {
	sheetName := "v2ray-packages"

	excelFile := excelize.NewFile()
	defer func() {
		if closeErr := excelFile.Close(); closeErr != nil {
			s.logger.Warn("failed to close excel file", zap.Error(closeErr))
		}
	}()

	sheetIndex, err := excelFile.NewSheet(sheetName)
	if err != nil {
		s.logger.Error("failed to create new sheet in excel file", zap.Error(err))
		return "", nil, err
	}

	headers := []string{"Customer Label", "Share Link", "Subscription Link"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := excelFile.SetColWidth(sheetName, cell[:1], cell[:1], 45); err != nil {
			s.logger.Error("failed to set column width", zap.Error(err))
			return "", nil, err
		}
		if err := excelFile.SetCellValue(sheetName, cell, header); err != nil {
			s.logger.Error("failed to set header cell value", zap.Error(err))
			return "", nil, err
		}
	}

	for idx, row := range rows {
		rowIndex := idx + 2
		labelCell, _ := excelize.CoordinatesToCellName(1, rowIndex)
		shareCell, _ := excelize.CoordinatesToCellName(2, rowIndex)
		subCell, _ := excelize.CoordinatesToCellName(3, rowIndex)

		if err := excelFile.SetCellValue(sheetName, labelCell, row.CustomerLabel); err != nil {
			return "", nil, err
		}
		if err := excelFile.SetCellValue(sheetName, shareCell, row.ShareLink); err != nil {
			return "", nil, err
		}
		if err := excelFile.SetCellValue(sheetName, subCell, row.SubscriptionLink); err != nil {
			return "", nil, err
		}
	}

	excelFile.SetActiveSheet(sheetIndex)
	if err := excelFile.DeleteSheet("Sheet1"); err != nil {
		s.logger.Warn("failed to delete default sheet", zap.Error(err))
	}

	f, err := os.CreateTemp("", "v2ray-packages-*.xlsx")
	if err != nil {
		s.logger.Error("failed to create temp file for v2ray package xlsx export", zap.Error(err))
		return "", nil, err
	}
	path := f.Name()
	f.Close()

	if err := excelFile.SaveAs(path); err != nil {
		s.logger.Error("failed to save excel file", zap.Error(err))
		os.Remove(path)
		return "", nil, err
	}

	return path, func() { os.Remove(path) }, nil
}
