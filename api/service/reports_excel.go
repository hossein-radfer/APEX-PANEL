package service

import (
	"bytes"
	"fmt"

	"go.uber.org/zap"

	"github.com/xuri/excelize/v2"
)

// GetExcelExport is report 14 -- a single workbook, one sheet per report,
// covering the exact same range-filtered data every chart on the Reports
// page shows, so the exported file always matches what's currently on
// screen. Returns raw .xlsx bytes rather than writing to a file path (see
// this package's own excel.go for the older, file-path-based precedent
// this deliberately does NOT follow -- writing to a shared relative path
// on disk is fragile under concurrent requests and unnecessary now that
// excelize can write directly to an in-memory buffer).
func (s *ReportsService) GetExcelExport(rangeParam string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Warn("failed to close excel file", zap.Error(err))
		}
	}()

	writeSheet := func(name string, headers []string, rows [][]interface{}) error {
		if _, err := file.NewSheet(name); err != nil {
			return fmt.Errorf("creating sheet %q: %w", name, err)
		}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			if err := file.SetCellValue(name, cell, h); err != nil {
				return err
			}
		}
		for r, row := range rows {
			for c, val := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
				if err := file.SetCellValue(name, cell, val); err != nil {
					return err
				}
			}
		}
		return nil
	}

	dailyUsage, err := s.GetDailyUsage(rangeParam, "")
	if err != nil {
		return nil, err
	}
	dailyRows := make([][]interface{}, 0, len(dailyUsage.Points))
	for _, p := range dailyUsage.Points {
		dailyRows = append(dailyRows, []interface{}{p.Date, p.WireGuardBytes, p.UserManagerBytes, p.V2RayBytes, p.TotalBytes})
	}
	if err := writeSheet("Daily Usage", []string{"Date", "WireGuard (bytes)", "User Manager (bytes)", "V2Ray (bytes)", "Total (bytes)"}, dailyRows); err != nil {
		return nil, err
	}

	resellerRanking, err := s.GetResellerUsageRanking(rangeParam)
	if err != nil {
		return nil, err
	}
	resellerRows := make([][]interface{}, 0, len(resellerRanking.Rows))
	for _, r := range resellerRanking.Rows {
		resellerRows = append(resellerRows, []interface{}{r.Name, r.Bytes})
	}
	if err := writeSheet("Reseller Ranking", []string{"Reseller", "Bytes"}, resellerRows); err != nil {
		return nil, err
	}

	activityRanking, err := s.GetResellerActivityRanking(rangeParam)
	if err != nil {
		return nil, err
	}
	activityRows := make([][]interface{}, 0, len(activityRanking.Rows))
	for _, r := range activityRanking.Rows {
		activityRows = append(activityRows, []interface{}{r.Name, r.Bytes, r.PackageCount})
	}
	if err := writeSheet("Reseller Activity", []string{"Reseller", "Bytes", "Package Count"}, activityRows); err != nil {
		return nil, err
	}

	protocolShare, err := s.GetProtocolShare(rangeParam)
	if err != nil {
		return nil, err
	}
	shareRows := [][]interface{}{
		{"WireGuard", protocolShare.WireGuardBytes},
		{"User Manager", protocolShare.UserManagerBytes},
		{"V2Ray", protocolShare.V2RayBytes},
	}
	if err := writeSheet("Protocol Share", []string{"Protocol", "Bytes"}, shareRows); err != nil {
		return nil, err
	}

	expiring, err := s.GetExpiringSoon()
	if err != nil {
		return nil, err
	}
	expiringRows := make([][]interface{}, 0, len(expiring.ExpiringBySoonDate))
	for _, e := range expiring.ExpiringBySoonDate {
		days := 0
		if e.DaysRemaining != nil {
			days = *e.DaysRemaining
		}
		expireAt := ""
		if e.ExpireAt != nil {
			expireAt = *e.ExpireAt
		}
		expiringRows = append(expiringRows, []interface{}{e.Name, e.Protocol, expireAt, days})
	}
	if err := writeSheet("Expiring Soon", []string{"Name", "Protocol", "Expire Date", "Days Remaining"}, expiringRows); err != nil {
		return nil, err
	}

	financial, err := s.GetFinancialReport(rangeParam, "day")
	if err != nil {
		return nil, err
	}
	financialRows := make([][]interface{}, 0, len(financial.Points))
	for _, p := range financial.Points {
		financialRows = append(financialRows, []interface{}{p.Bucket, p.ChargeAmount, p.DebitAmount})
	}
	if err := writeSheet("Financial", []string{"Date", "Charge Amount", "Debit Amount"}, financialRows); err != nil {
		return nil, err
	}

	file.SetActiveSheet(0)
	if err := file.DeleteSheet("Sheet1"); err != nil {
		s.logger.Warn("failed to delete default sheet from export workbook", zap.Error(err))
	}

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		s.logger.Error("failed to write excel export buffer", zap.Error(err))
		return nil, err
	}

	return buf.Bytes(), nil
}
