package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// AccountingService implements the admin's own private profit/loss
// bookkeeping -- see model/accounting.go's own doc comment for the full
// design rationale and why this is deliberately independent from Wallet/
// LedgerEntry/Invoice/PricePlan.
type AccountingService struct {
	db          *gorm.DB
	botNotifier *BotNotifier
	settings    *BotSettingsService
	logger      *zap.Logger
}

func NewAccountingService(db *gorm.DB) *AccountingService {
	return &AccountingService{
		db:       db,
		settings: NewBotSettingsService(db),
		logger:   zap.L().Named("AccountingService"),
	}
}

// SetBotNotifier wires the recurring-cost-reminder delivery channel in --
// mirrors every other service's identical SetBotNotifier(botNotifier)
// pattern (see http-server.go's construction order).
func (s *AccountingService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// Tick is the scheduled job entry point (see cmd/main.go's gocron
// registration) -- once a day, it finds every recurring cost that has come
// due and hasn't been reminded about yet, sends one Telegram notification
// per cost, and marks it reminded so the next tick doesn't repeat it.
// Reminders resume automatically the next time AdvanceRecurringCost (or
// the admin manually marking a cost paid) clears ReminderSentAt for that
// cost's NEXT due date.
func (s *AccountingService) Tick() {
	if s.botNotifier == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}

	due, err := s.DueRecurringCosts(time.Now())
	if err != nil {
		s.logger.Error("failed to check due recurring costs", zap.Error(err))
		return
	}
	if len(due) == 0 {
		return
	}

	partnerNames, err := s.PartnerNamesByID()
	if err != nil {
		s.logger.Error("failed to load partner names for due-cost reminders", zap.Error(err))
		return
	}

	for _, cost := range due {
		dueDate := ""
		if cost.NextDueAt != nil {
			dueDate = cost.NextDueAt.Format("2006-01-02")
		}
		s.botNotifier.NotifyRecurringCostDue(settings, partnerNames[cost.PartnerID], cost.AmountToman, dueDate)
		if err := s.MarkReminderSent(cost.ID); err != nil {
			s.logger.Warn("failed to mark recurring cost reminder as sent", zap.Uint("cost_id", cost.ID), zap.Error(err))
		}
	}
}

var accountingReceiptsPath string

func init() {
	appCfg := config.GetAppConfig()
	accountingReceiptsPath = filepath.Join(appCfg.PeerFilesDir, "accounting-receipts")
	if err := os.MkdirAll(accountingReceiptsPath, os.ModePerm); err != nil {
		zap.L().Named("AccountingService").Error("failed to create accounting receipts directory", zap.Error(err))
	}
}

// --- Partners ---

func (s *AccountingService) ListPartners() ([]model.AccountingPartner, error) {
	var partners []model.AccountingPartner
	if err := s.db.Order("id desc").Find(&partners).Error; err != nil {
		s.logger.Error("failed to list partners", zap.Error(err))
		return nil, err
	}
	return partners, nil
}

func (s *AccountingService) CreatePartner(name string, comment *string) (*model.AccountingPartner, error) {
	partner := model.AccountingPartner{Name: name, Comment: comment}
	if err := s.db.Create(&partner).Error; err != nil {
		s.logger.Error("failed to create partner", zap.Error(err))
		return nil, err
	}
	return &partner, nil
}

func (s *AccountingService) UpdatePartner(id uint, name string, comment *string) (*model.AccountingPartner, error) {
	var partner model.AccountingPartner
	if err := s.db.First(&partner, id).Error; err != nil {
		return nil, fmt.Errorf("partner not found: %w", err)
	}
	partner.Name = name
	partner.Comment = comment
	if err := s.db.Save(&partner).Error; err != nil {
		s.logger.Error("failed to update partner", zap.Error(err))
		return nil, err
	}
	return &partner, nil
}

// DeletePartner is blocked while any AccountingCost still references this
// partner -- a hard foreign-key-style guard rather than a cascading
// delete, since silently deleting an entire cost history because someone
// removed a partner row would corrupt the admin's own profit/loss numbers
// for every past period, not just going forward.
func (s *AccountingService) DeletePartner(id uint) error {
	var costCount int64
	if err := s.db.Model(&model.AccountingCost{}).Where("partner_id = ?", id).Count(&costCount).Error; err != nil {
		return err
	}
	if costCount > 0 {
		return fmt.Errorf("cannot delete partner with %d existing cost record(s)", costCount)
	}
	if err := s.db.Delete(&model.AccountingPartner{}, id).Error; err != nil {
		s.logger.Error("failed to delete partner", zap.Error(err))
		return err
	}
	return nil
}

// --- Costs ---

type CreateCostInput struct {
	PartnerID              uint
	ServerID               *uint
	Protocol               *string
	LocationKey            *string
	AmountToman            int64
	Description            *string
	PaidAt                 *time.Time
	DueAt                  *time.Time
	RecurrenceIntervalDays *int
}

func (s *AccountingService) ListCosts() ([]model.AccountingCost, error) {
	var costs []model.AccountingCost
	if err := s.db.Order("id desc").Find(&costs).Error; err != nil {
		s.logger.Error("failed to list costs", zap.Error(err))
		return nil, err
	}
	return costs, nil
}

// PartnerNamesByID/ServerNamesByID are small lookup helpers the HTTP layer
// uses to enrich AccountingCostResponse with PartnerName/ServerName without
// N+1 querying per row -- one bulk load each, same convention as
// WgPeer.locationLabelsByKey's "load everything into a map once" pattern.
func (s *AccountingService) PartnerNamesByID() (map[uint]string, error) {
	var partners []model.AccountingPartner
	if err := s.db.Find(&partners).Error; err != nil {
		return nil, err
	}
	m := make(map[uint]string, len(partners))
	for _, p := range partners {
		m[p.ID] = p.Name
	}
	return m, nil
}

// PartnerName resolves a single partner's name -- used by the create/update
// cost handlers, which only need one lookup rather than PartnerNamesByID's
// bulk map (that one's for ListCosts, where N+1 queries would actually
// matter).
func (s *AccountingService) PartnerName(id uint) (string, error) {
	var partner model.AccountingPartner
	if err := s.db.First(&partner, id).Error; err != nil {
		return "", err
	}
	return partner.Name, nil
}

// ServerName resolves a single server's name -- see PartnerName's own doc
// comment for why this is separate from the bulk ServerNamesByID.
func (s *AccountingService) ServerName(id uint) (string, error) {
	var server model.Server
	if err := s.db.First(&server, id).Error; err != nil {
		return "", err
	}
	return server.Name, nil
}

func (s *AccountingService) ServerNamesByID() (map[uint]string, error) {
	var servers []model.Server
	if err := s.db.Find(&servers).Error; err != nil {
		return nil, err
	}
	m := make(map[uint]string, len(servers))
	for _, srv := range servers {
		m[srv.ID] = srv.Name
	}
	return m, nil
}

func (s *AccountingService) CreateCost(input CreateCostInput) (*model.AccountingCost, error) {
	var partner model.AccountingPartner
	if err := s.db.First(&partner, input.PartnerID).Error; err != nil {
		return nil, fmt.Errorf("partner not found: %w", err)
	}

	cost := model.AccountingCost{
		PartnerID:              input.PartnerID,
		ServerID:               input.ServerID,
		Protocol:               input.Protocol,
		LocationKey:            input.LocationKey,
		AmountToman:            input.AmountToman,
		Description:            input.Description,
		PaidAt:                 input.PaidAt,
		DueAt:                  input.DueAt,
		RecurrenceIntervalDays: input.RecurrenceIntervalDays,
	}
	// A recurring cost's first NextDueAt is DueAt itself (or now, if no
	// DueAt was given) -- AccountingScheduler advances it from there every
	// time it fires, so the very first reminder lands on the date the
	// admin actually specified, not one interval later.
	if input.RecurrenceIntervalDays != nil {
		if input.DueAt != nil {
			cost.NextDueAt = input.DueAt
		} else {
			now := time.Now()
			cost.NextDueAt = &now
		}
	}

	if err := s.db.Create(&cost).Error; err != nil {
		s.logger.Error("failed to create cost", zap.Error(err))
		return nil, err
	}
	return &cost, nil
}

func (s *AccountingService) UpdateCost(id uint, input CreateCostInput) (*model.AccountingCost, error) {
	var cost model.AccountingCost
	if err := s.db.First(&cost, id).Error; err != nil {
		return nil, fmt.Errorf("cost not found: %w", err)
	}

	cost.PartnerID = input.PartnerID
	cost.ServerID = input.ServerID
	cost.Protocol = input.Protocol
	cost.LocationKey = input.LocationKey
	cost.AmountToman = input.AmountToman
	cost.Description = input.Description
	cost.PaidAt = input.PaidAt
	cost.DueAt = input.DueAt
	cost.RecurrenceIntervalDays = input.RecurrenceIntervalDays

	if err := s.db.Save(&cost).Error; err != nil {
		s.logger.Error("failed to update cost", zap.Error(err))
		return nil, err
	}
	return &cost, nil
}

func (s *AccountingService) DeleteCost(id uint) error {
	if err := s.db.Delete(&model.AccountingCost{}, id).Error; err != nil {
		s.logger.Error("failed to delete cost", zap.Error(err))
		return err
	}
	return nil
}

// AdvanceRecurringCost pushes a recurring cost's NextDueAt forward by its
// own RecurrenceIntervalDays and clears ReminderSentAt -- called by
// AccountingScheduler once a due reminder has actually been sent, and
// exposed here too so the admin can manually mark a recurring cost as
// "renewed" from the UI without waiting for the scheduler's own tick.
func (s *AccountingService) AdvanceRecurringCost(id uint) error {
	var cost model.AccountingCost
	if err := s.db.First(&cost, id).Error; err != nil {
		return fmt.Errorf("cost not found: %w", err)
	}
	if cost.RecurrenceIntervalDays == nil || cost.NextDueAt == nil {
		return fmt.Errorf("cost %d is not a recurring cost", id)
	}

	next := cost.NextDueAt.AddDate(0, 0, *cost.RecurrenceIntervalDays)
	now := time.Now()
	return s.db.Model(&cost).Updates(map[string]interface{}{
		"next_due_at":      next,
		"paid_at":          now,
		"reminder_sent_at": nil,
	}).Error
}

// --- Customer payments ---

type CreatePaymentInput struct {
	CustomerLabel string
	AmountToman   int64
	CostToman     int64
	Protocol      *string
	ResourceID    *uint
	Note          *string
	PaidAt        time.Time
}

// resolveLocationKey looks up the CURRENT location key for (protocol,
// resourceID) at the moment a sale is recorded -- see
// AccountingCustomerPayment.LocationKey's own doc comment for why this is
// denormalized onto the sale row rather than re-derived later. Returns nil
// (not an error) for an unrecognized protocol or a resource that no longer
// exists, since a sale must never fail to save just because its location
// can't be resolved right now.
func (s *AccountingService) resolveLocationKey(protocol *string, resourceID *uint) *string {
	if protocol == nil || resourceID == nil {
		return nil
	}

	switch *protocol {
	case model.AccountingSaleWireGuard:
		var peer model.Peer
		if err := s.db.First(&peer, *resourceID).Error; err != nil {
			return nil
		}
		var iface model.Interface
		if err := s.db.Where("name = ?", peer.Interface).First(&iface).Error; err != nil {
			return nil
		}
		key := fmt.Sprintf("%d", iface.ID)
		return &key
	case model.AccountingSaleUserManager:
		var account model.UserManagerAccount
		if err := s.db.First(&account, *resourceID).Error; err != nil {
			return nil
		}
		return &account.Group
	case model.AccountingSaleV2Ray:
		var loc model.V2RayPackageLocation
		if err := s.db.Where("package_id = ?", *resourceID).Order("id").First(&loc).Error; err != nil {
			return nil
		}
		key := fmt.Sprintf("%d", loc.PanelID)
		return &key
	default:
		return nil
	}
}

func (s *AccountingService) ListPayments() ([]model.AccountingCustomerPayment, error) {
	var payments []model.AccountingCustomerPayment
	if err := s.db.Order("id desc").Find(&payments).Error; err != nil {
		s.logger.Error("failed to list payments", zap.Error(err))
		return nil, err
	}
	return payments, nil
}

func (s *AccountingService) CreatePayment(input CreatePaymentInput) (*model.AccountingCustomerPayment, error) {
	payment := model.AccountingCustomerPayment{
		CustomerLabel: input.CustomerLabel,
		AmountToman:   input.AmountToman,
		CostToman:     input.CostToman,
		Protocol:      input.Protocol,
		ResourceID:    input.ResourceID,
		LocationKey:   s.resolveLocationKey(input.Protocol, input.ResourceID),
		Note:          input.Note,
		PaidAt:        input.PaidAt,
	}
	if err := s.db.Create(&payment).Error; err != nil {
		s.logger.Error("failed to create payment", zap.Error(err))
		return nil, err
	}
	return &payment, nil
}

func (s *AccountingService) UpdatePayment(id uint, input CreatePaymentInput) (*model.AccountingCustomerPayment, error) {
	var payment model.AccountingCustomerPayment
	if err := s.db.First(&payment, id).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}
	payment.CustomerLabel = input.CustomerLabel
	payment.AmountToman = input.AmountToman
	payment.CostToman = input.CostToman
	payment.Protocol = input.Protocol
	payment.ResourceID = input.ResourceID
	payment.LocationKey = s.resolveLocationKey(input.Protocol, input.ResourceID)
	payment.Note = input.Note
	payment.PaidAt = input.PaidAt
	if err := s.db.Save(&payment).Error; err != nil {
		s.logger.Error("failed to update payment", zap.Error(err))
		return nil, err
	}
	return &payment, nil
}

// DeletePayment also best-effort removes the payment's receipt file (if
// any) from disk -- logged, not fatal, mirroring WgPeer.DeletePeer's own
// "a file-cleanup failure never blocks the actual delete" convention.
func (s *AccountingService) DeletePayment(id uint) error {
	var payment model.AccountingCustomerPayment
	if err := s.db.First(&payment, id).Error; err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}
	if err := s.db.Delete(&payment).Error; err != nil {
		s.logger.Error("failed to delete payment", zap.Error(err))
		return err
	}
	if payment.ReceiptFilePath != nil {
		if err := os.Remove(*payment.ReceiptFilePath); err != nil {
			s.logger.Warn("failed to remove receipt file for deleted payment", zap.String("path", *payment.ReceiptFilePath), zap.Error(err))
		}
	}
	return nil
}

// SaveReceiptFile writes the given bytes to disk under a name derived from
// the payment's own ID (so a repeat upload for the same payment simply
// overwrites the previous receipt) and records the path on the payment
// row -- ext should include the leading dot (e.g. ".jpg").
func (s *AccountingService) SaveReceiptFile(paymentID uint, ext string, data []byte) (string, error) {
	var payment model.AccountingCustomerPayment
	if err := s.db.First(&payment, paymentID).Error; err != nil {
		return "", fmt.Errorf("payment not found: %w", err)
	}

	path := filepath.Join(accountingReceiptsPath, fmt.Sprintf("%d%s", paymentID, ext))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		s.logger.Error("failed to write receipt file", zap.Error(err))
		return "", err
	}

	if err := s.db.Model(&payment).Update("receipt_file_path", path).Error; err != nil {
		s.logger.Error("failed to persist receipt file path", zap.Error(err))
		return "", err
	}
	return path, nil
}

func (s *AccountingService) GetReceiptFilePath(paymentID uint) (string, error) {
	var payment model.AccountingCustomerPayment
	if err := s.db.First(&payment, paymentID).Error; err != nil {
		return "", fmt.Errorf("payment not found: %w", err)
	}
	if payment.ReceiptFilePath == nil {
		return "", fmt.Errorf("payment %d has no receipt file", paymentID)
	}
	return *payment.ReceiptFilePath, nil
}

// --- Summary / dashboard ---

// PeriodSummary is one bucket (e.g. one calendar day or month) of the
// income-vs-cost chart the dashboard renders.
type PeriodSummary struct {
	Period      string `json:"period"` // "2026-08-30" or "2026-08"
	IncomeToman int64  `json:"income_toman"`
	CostToman   int64  `json:"cost_toman"`
}

// AccountingSummary is the full payload GetSummary returns -- aggregate
// totals plus a time-bucketed series for the dashboard chart.
type AccountingSummary struct {
	TotalIncomeToman int64           `json:"total_income_toman"`
	TotalCostToman   int64           `json:"total_cost_toman"`
	ProfitToman      int64           `json:"profit_toman"`
	Series           []PeriodSummary `json:"series"`
}

// GetSummary computes total income/cost/profit within [since, until] and a
// daily-bucketed series for charting -- costs are counted by PaidAt (an
// unpaid/future-due cost with PaidAt nil contributes nothing yet, matching
// "profit" meaning actual cash flow, not accrued/pending obligations).
func (s *AccountingService) GetSummary(since, until time.Time) (*AccountingSummary, error) {
	var payments []model.AccountingCustomerPayment
	if err := s.db.Where("paid_at BETWEEN ? AND ?", since, until).Find(&payments).Error; err != nil {
		s.logger.Error("failed to load payments for summary", zap.Error(err))
		return nil, err
	}

	var costs []model.AccountingCost
	if err := s.db.Where("paid_at IS NOT NULL AND paid_at BETWEEN ? AND ?", since, until).Find(&costs).Error; err != nil {
		s.logger.Error("failed to load costs for summary", zap.Error(err))
		return nil, err
	}

	byDay := make(map[string]*PeriodSummary)
	getBucket := func(day string) *PeriodSummary {
		if b, ok := byDay[day]; ok {
			return b
		}
		b := &PeriodSummary{Period: day}
		byDay[day] = b
		return b
	}

	var totalIncome, totalCost int64
	for _, p := range payments {
		totalIncome += p.AmountToman
		getBucket(p.PaidAt.Format("2006-01-02")).IncomeToman += p.AmountToman
	}
	for _, c := range costs {
		totalCost += c.AmountToman
		getBucket(c.PaidAt.Format("2006-01-02")).CostToman += c.AmountToman
	}

	series := make([]PeriodSummary, 0, len(byDay))
	for _, b := range byDay {
		series = append(series, *b)
	}
	// Chronological order for the chart -- map iteration order is random.
	for i := 1; i < len(series); i++ {
		for j := i; j > 0 && series[j-1].Period > series[j].Period; j-- {
			series[j-1], series[j] = series[j], series[j-1]
		}
	}

	return &AccountingSummary{
		TotalIncomeToman: totalIncome,
		TotalCostToman:   totalCost,
		ProfitToman:      totalIncome - totalCost,
		Series:           series,
	}, nil
}

// DueRecurringCosts returns every recurring cost whose NextDueAt has
// already passed and hasn't been reminded about yet -- used by
// AccountingScheduler's own ticking job.
func (s *AccountingService) DueRecurringCosts(now time.Time) ([]model.AccountingCost, error) {
	var costs []model.AccountingCost
	err := s.db.Where("recurrence_interval_days IS NOT NULL AND next_due_at <= ? AND reminder_sent_at IS NULL", now).Find(&costs).Error
	if err != nil {
		s.logger.Error("failed to load due recurring costs", zap.Error(err))
		return nil, err
	}
	return costs, nil
}

func (s *AccountingService) MarkReminderSent(costID uint) error {
	now := time.Now()
	return s.db.Model(&model.AccountingCost{}).Where("id = ?", costID).Update("reminder_sent_at", now).Error
}

// SellableResource is one Peer/UserManagerAccount/V2RayPackage the admin
// can pick as "what was sold" when logging a payment -- Name is the
// customer-facing label to show in the picker (Peer.Name/
// UserManagerAccount.Username/V2RayPackage.CustomerLabel, whichever that
// protocol actually has).
type SellableResource struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

// ListSellableResources returns every Peer/UserManagerAccount/V2RayPackage
// for the given protocol, for the payment-creation UI's resource picker.
func (s *AccountingService) ListSellableResources(protocol string) ([]SellableResource, error) {
	var result []SellableResource

	switch protocol {
	case model.AccountingSaleWireGuard:
		var peers []model.Peer
		if err := s.db.Order("id desc").Find(&peers).Error; err != nil {
			return nil, err
		}
		for _, p := range peers {
			result = append(result, SellableResource{Id: p.ID, Name: p.Name})
		}
	case model.AccountingSaleUserManager:
		var accounts []model.UserManagerAccount
		if err := s.db.Order("id desc").Find(&accounts).Error; err != nil {
			return nil, err
		}
		for _, a := range accounts {
			result = append(result, SellableResource{Id: a.ID, Name: a.Username})
		}
	case model.AccountingSaleV2Ray:
		var packages []model.V2RayPackage
		if err := s.db.Order("id desc").Find(&packages).Error; err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			name := pkg.UUID
			if pkg.CustomerLabel != nil && *pkg.CustomerLabel != "" {
				name = *pkg.CustomerLabel
			}
			result = append(result, SellableResource{Id: pkg.ID, Name: name})
		}
	default:
		return nil, fmt.Errorf("unknown protocol %q", protocol)
	}

	if result == nil {
		result = []SellableResource{}
	}
	return result, nil
}

// --- Per-user / per-location profitability ---

// UserProfitability is one sold resource's own profit line -- the admin's
// own explicit requirement: "برای هر یوزر ... بگم چقدر فروختم و سود رو
// محاسبه کنه" (for every user, let me say how much I sold it for and
// compute the profit). ProfitToman is a plain subtraction, not a query
// aggregate, since AmountToman/CostToman both live on the same
// AccountingCustomerPayment row (see that model's own doc comment on why
// cost-per-sale is captured at sale time rather than derived from
// AccountingCost, which tracks the admin's SUPPLIER-side costs at the
// partner/location level, not a per-customer cost basis).
type UserProfitability struct {
	PaymentID     uint    `json:"payment_id"`
	CustomerLabel string  `json:"customer_label"`
	Protocol      *string `json:"protocol,omitempty"`
	ResourceID    *uint   `json:"resource_id,omitempty"`
	LocationKey   *string `json:"location_key,omitempty"`
	LocationLabel *string `json:"location_label,omitempty"`
	AmountToman   int64   `json:"amount_toman"`
	CostToman     int64   `json:"cost_toman"`
	ProfitToman   int64   `json:"profit_toman"`
	PaidAt        string  `json:"paid_at"`
}

// GetUserProfitability lists every sale within [since, until] with its own
// sale-price/cost/profit -- LocationLabel resolves through
// ApplicationResourceLocation exactly like the share-page/dashboard
// location labels already do (see WgPeer.locationLabelsByKey's identical
// pattern), falling back to nil (raw LocationKey shown instead) when the
// admin hasn't labeled that interface/group/panel.
func (s *AccountingService) GetUserProfitability(since, until time.Time) ([]UserProfitability, error) {
	var payments []model.AccountingCustomerPayment
	if err := s.db.Where("paid_at BETWEEN ? AND ?", since, until).Order("paid_at desc").Find(&payments).Error; err != nil {
		s.logger.Error("failed to load payments for user profitability", zap.Error(err))
		return nil, err
	}

	labels := s.locationLabelsByKey()

	result := make([]UserProfitability, 0, len(payments))
	for _, p := range payments {
		row := UserProfitability{
			PaymentID:     p.ID,
			CustomerLabel: p.CustomerLabel,
			Protocol:      p.Protocol,
			ResourceID:    p.ResourceID,
			LocationKey:   p.LocationKey,
			AmountToman:   p.AmountToman,
			CostToman:     p.CostToman,
			ProfitToman:   p.AmountToman - p.CostToman,
			PaidAt:        p.PaidAt.Format("2006-01-02"),
		}
		if p.Protocol != nil && p.LocationKey != nil {
			row.LocationLabel = lookupLabel(labels, accountingResourceType(*p.Protocol), *p.LocationKey)
		}
		result = append(result, row)
	}
	return result, nil
}

// LocationProfitability is one location's (interface/group/panel) rolled-up
// revenue/cost/profit -- the admin's own explicit requirement: "هر لوکشین
// چقدر در یماد برام" (how much does each location bring in for me).
type LocationProfitability struct {
	Protocol      string  `json:"protocol"`
	LocationKey   string  `json:"location_key"`
	LocationLabel *string `json:"location_label,omitempty"`
	IncomeToman   int64   `json:"income_toman"`
	CostToman     int64   `json:"cost_toman"`
	ProfitToman   int64   `json:"profit_toman"`
}

// LocationLabelFor resolves the admin-configured label for one (protocol,
// locationKey) pair via the shared ApplicationResourceLocation table --
// used by the HTTP layer to enrich a single cost's response (ListCosts'
// own bulk path uses locationLabelsByKey directly to avoid N+1 queries).
func (s *AccountingService) LocationLabelFor(protocol, locationKey string) *string {
	labels := s.locationLabelsByKey()
	return lookupLabel(labels, accountingResourceType(protocol), locationKey)
}

// accountingResourceType maps an AccountingSale* protocol constant to the
// matching ApplicationResourceLocation ResourceType constant -- the two
// enums are deliberately kept as separate constants (accounting is not
// coupled to the application-sharing feature), so this is the one place
// that translates between them.
func accountingResourceType(protocol string) string {
	switch protocol {
	case model.AccountingSaleWireGuard:
		return model.ResourceTypeWireGuardInterface
	case model.AccountingSaleUserManager:
		return model.ResourceTypeUserManagerGroup
	case model.AccountingSaleV2Ray:
		return model.ResourceTypeXuiPanel
	default:
		return ""
	}
}

// locationLabelsByKey duplicates WgPeer/ApplicationService's identical
// helper of the same name -- all three read the exact same shared
// ApplicationResourceLocation table (see that model's own doc comment);
// duplicated rather than shared across services to keep AccountingService
// independent (matches this codebase's own established precedent, e.g.
// WgPeer's copy of ApplicationService's original).
func (s *AccountingService) locationLabelsByKey() map[string]string {
	var rows []model.ApplicationResourceLocation
	s.db.Find(&rows)
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		result[locationKey(r.ResourceType, r.ResourceKey)] = r.Label
	}
	return result
}

// GetLocationProfitability rolls sales and costs up to the (protocol,
// location) level within [since, until] -- sales grouped by their own
// denormalized LocationKey, costs by THEIRS (see AccountingCost.
// LocationKey's own doc comment for why costs need this same key rather
// than just ServerID). A location with sales but no matching costs still
// appears (100% margin, correctly); a location with only costs and no
// sales yet also appears (negative profit, correctly flagging a
// not-yet-profitable location).
func (s *AccountingService) GetLocationProfitability(since, until time.Time) ([]LocationProfitability, error) {
	var payments []model.AccountingCustomerPayment
	if err := s.db.Where("paid_at BETWEEN ? AND ? AND protocol IS NOT NULL AND location_key IS NOT NULL", since, until).Find(&payments).Error; err != nil {
		return nil, err
	}
	var costs []model.AccountingCost
	if err := s.db.Where("paid_at IS NOT NULL AND paid_at BETWEEN ? AND ? AND protocol IS NOT NULL AND location_key IS NOT NULL", since, until).Find(&costs).Error; err != nil {
		return nil, err
	}

	type key struct{ protocol, location string }
	byKey := make(map[key]*LocationProfitability)
	getBucket := func(protocol, location string) *LocationProfitability {
		k := key{protocol, location}
		if b, ok := byKey[k]; ok {
			return b
		}
		b := &LocationProfitability{Protocol: protocol, LocationKey: location}
		byKey[k] = b
		return b
	}

	for _, p := range payments {
		getBucket(*p.Protocol, *p.LocationKey).IncomeToman += p.AmountToman
	}
	for _, c := range costs {
		getBucket(*c.Protocol, *c.LocationKey).CostToman += c.AmountToman
	}

	labels := s.locationLabelsByKey()
	result := make([]LocationProfitability, 0, len(byKey))
	for _, b := range byKey {
		b.ProfitToman = b.IncomeToman - b.CostToman
		b.LocationLabel = lookupLabel(labels, accountingResourceType(b.Protocol), b.LocationKey)
		result = append(result, *b)
	}
	return result, nil
}
