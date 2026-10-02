package schema

type AccountingPartnerResponse struct {
	Id      uint    `json:"id"`
	Name    string  `json:"name"`
	Comment *string `json:"comment"`
}

type CreatePartnerRequest struct {
	Name    string  `json:"name" validate:"required"`
	Comment *string `json:"comment,omitempty"`
}

type AccountingCostResponse struct {
	Id                     uint    `json:"id"`
	PartnerID              uint    `json:"partner_id"`
	PartnerName            string  `json:"partner_name"`
	ServerID               *uint   `json:"server_id,omitempty"`
	ServerName             *string `json:"server_name,omitempty"`
	Protocol               *string `json:"protocol,omitempty"`
	LocationKey            *string `json:"location_key,omitempty"`
	LocationLabel          *string `json:"location_label,omitempty"`
	AmountToman            int64   `json:"amount_toman"`
	Description            *string `json:"description,omitempty"`
	PaidAt                 *string `json:"paid_at,omitempty"`
	DueAt                  *string `json:"due_at,omitempty"`
	RecurrenceIntervalDays *int    `json:"recurrence_interval_days,omitempty"`
	NextDueAt              *string `json:"next_due_at,omitempty"`
}

type CreateCostRequest struct {
	PartnerID              uint    `json:"partner_id" validate:"required"`
	ServerID               *uint   `json:"server_id,omitempty"`
	Protocol               *string `json:"protocol,omitempty" validate:"omitempty,oneof=wireguard user_manager v2ray"`
	LocationKey            *string `json:"location_key,omitempty"`
	AmountToman            int64   `json:"amount_toman" validate:"required,min=1"`
	Description            *string `json:"description,omitempty"`
	PaidAt                 *string `json:"paid_at,omitempty"`
	DueAt                  *string `json:"due_at,omitempty"`
	RecurrenceIntervalDays *int    `json:"recurrence_interval_days,omitempty" validate:"omitempty,min=1"`
}

type AccountingPaymentResponse struct {
	Id            uint    `json:"id"`
	CustomerLabel string  `json:"customer_label"`
	AmountToman   int64   `json:"amount_toman"`
	CostToman     int64   `json:"cost_toman"`
	ProfitToman   int64   `json:"profit_toman"`
	Protocol      *string `json:"protocol,omitempty"`
	ResourceID    *uint   `json:"resource_id,omitempty"`
	LocationKey   *string `json:"location_key,omitempty"`
	LocationLabel *string `json:"location_label,omitempty"`
	Note          *string `json:"note,omitempty"`
	HasReceipt    bool    `json:"has_receipt"`
	PaidAt        string  `json:"paid_at"`
}

type CreatePaymentRequest struct {
	CustomerLabel string  `json:"customer_label" validate:"required"`
	AmountToman   int64   `json:"amount_toman" validate:"required,min=1"`
	CostToman     int64   `json:"cost_toman,omitempty"`
	Protocol      *string `json:"protocol,omitempty" validate:"omitempty,oneof=wireguard user_manager v2ray"`
	ResourceID    *uint   `json:"resource_id,omitempty"`
	Note          *string `json:"note,omitempty"`
	PaidAt        string  `json:"paid_at" validate:"required"`
}

type SellableResourceResponse struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type UserProfitabilityResponse struct {
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

type LocationProfitabilityResponse struct {
	Protocol      string  `json:"protocol"`
	LocationKey   string  `json:"location_key"`
	LocationLabel *string `json:"location_label,omitempty"`
	IncomeToman   int64   `json:"income_toman"`
	CostToman     int64   `json:"cost_toman"`
	ProfitToman   int64   `json:"profit_toman"`
}

type AccountingPeriodSummary struct {
	Period      string `json:"period"`
	IncomeToman int64  `json:"income_toman"`
	CostToman   int64  `json:"cost_toman"`
}

type AccountingSummaryResponse struct {
	TotalIncomeToman int64                     `json:"total_income_toman"`
	TotalCostToman   int64                     `json:"total_cost_toman"`
	ProfitToman      int64                     `json:"profit_toman"`
	Series           []AccountingPeriodSummary `json:"series"`
}
