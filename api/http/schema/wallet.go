package schema

type WalletResponse struct {
	ResellerID    uint   `json:"reseller_id"`
	BalanceAmount int64  `json:"balance_amount"`
	IsFrozen      bool   `json:"is_frozen"`
	FrozenReason  *string `json:"frozen_reason"`
}

type LedgerEntryResponse struct {
	ID            uint    `json:"id"`
	ResellerID    uint    `json:"reseller_id"`
	TransactionID string  `json:"transaction_id"`
	EntryType     string  `json:"entry_type"`
	Amount        int64   `json:"amount"`
	BalanceAfter  int64   `json:"balance_after"`
	Description   string  `json:"description"`
	ReferenceType *string `json:"reference_type"`
	ReferenceID   *uint   `json:"reference_id"`
	CreatedAt     int64   `json:"created_at"`
}

type CreditRequest struct {
	Amount      int64   `json:"amount" validate:"required,gt=0"`
	Description string  `json:"description"`
	ReferenceID *uint   `json:"reference_id"`
}

type DebitRequest struct {
	Amount      int64   `json:"amount" validate:"required,gt=0"`
	Description string  `json:"description"`
	ReferenceID *uint   `json:"reference_id"`
}

type FreezeWalletRequest struct {
	Reason string `json:"reason" validate:"required"`
}

type PricePlanResponse struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	Description      *string `json:"description"`
	BasePriceAmount  int64   `json:"base_price_amount"`
	BillingInterval  string  `json:"billing_interval"`
	TrafficAllowance *int64  `json:"traffic_allowance"`
	MaxPeers         *int32  `json:"max_peers"`
	MaxServers       *int32  `json:"max_servers"`
	IsActive         bool    `json:"is_active"`
}

type CreatePricePlanRequest struct {
	Name        string  `json:"name" validate:"required,min=1"`
	Description *string `json:"description"`
	// BasePriceAmount deliberately does NOT use `required` -- see the
	// identical, confirmed bug fixed in
	// schema.CreateV2RayTrafficPackageRequest (go-playground/validator's
	// `required` wrongly rejects a valid BasePriceAmount: 0 free plan as
	// "missing"). `min=0` alone still enforces non-negative.
	BasePriceAmount  int64   `json:"base_price_amount" validate:"min=0"`
	BillingInterval  string  `json:"billing_interval" validate:"required,oneof=MONTHLY QUARTERLY ANNUAL PAY_AS_YOU_GO"`
	TrafficAllowance *int64  `json:"traffic_allowance"`
	MaxPeers         *int32  `json:"max_peers"`
	MaxServers       *int32  `json:"max_servers"`
}

type UpdatePricePlanRequest struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	BasePriceAmount  *int64  `json:"base_price_amount" validate:"omitempty,min=0"`
	BillingInterval  *string `json:"billing_interval" validate:"omitempty,oneof=MONTHLY QUARTERLY ANNUAL PAY_AS_YOU_GO"`
	TrafficAllowance *int64  `json:"traffic_allowance"`
	MaxPeers         *int32  `json:"max_peers"`
	MaxServers       *int32  `json:"max_servers"`
}
