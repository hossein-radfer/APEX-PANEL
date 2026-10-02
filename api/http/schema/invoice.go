package schema

type InvoiceResponse struct {
	ID            uint    `json:"id"`
	ResellerID    uint    `json:"reseller_id"`
	InvoiceNumber string  `json:"invoice_number"`
	Status        string  `json:"status"`
	Amount        int64   `json:"amount"`
	Description   string  `json:"description"`
	PeriodStart   int64   `json:"period_start"`
	PeriodEnd     int64   `json:"period_end"`
	IssuedAt      *int64  `json:"issued_at"`
	DueDate       *int64  `json:"due_date"`
	PaidAt        *int64  `json:"paid_at"`
	Notes         *string `json:"notes"`
	CreatedAt     int64   `json:"created_at"`
}

type CreateInvoiceRequest struct {
	Amount      int64   `json:"amount" validate:"required,gt=0"`
	Description string  `json:"description"`
	PeriodStart *int64  `json:"period_start"`
	PeriodEnd   *int64  `json:"period_end"`
	DueDate     *int64  `json:"due_date"`
	Notes       *string `json:"notes"`
	AutoIssue   bool    `json:"auto_issue"`
}
