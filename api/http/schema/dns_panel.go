package schema

type CreateDNSPanelRequest struct {
	Name       string `json:"name" validate:"required"`
	SaleTitle  string `json:"sale_title" validate:"required"`
	APIBaseURL string `json:"api_base_url" validate:"required"`
	APIKey     string `json:"api_key" validate:"required"`
}

// TestDNSPanelConnectionRequest is the minimal field set needed to test
// connectivity BEFORE a panel has been saved, mirroring
// TestXuiPanelConnectionRequest's identical role.
type TestDNSPanelConnectionRequest struct {
	APIBaseURL string `json:"api_base_url" validate:"required"`
	APIKey     string `json:"api_key" validate:"required"`
}

type UpdateDNSPanelRequest struct {
	Name       string  `json:"name,omitempty"`
	SaleTitle  string  `json:"sale_title,omitempty"`
	APIBaseURL string  `json:"api_base_url,omitempty"`
	APIKey     *string `json:"api_key,omitempty"` // omitted/nil leaves the existing key unchanged
}

type DNSPanelResponse struct {
	Id           uint    `json:"id"`
	Name         string  `json:"name"`
	SaleTitle    string  `json:"sale_title"`
	APIBaseURL   string  `json:"api_base_url"`
	Status       string  `json:"status"`
	LastError    *string `json:"last_error,omitempty"`
	LastSyncedAt *string `json:"last_synced_at,omitempty"`
}

// DNSPanelSummary is the minimal, reseller-safe view of a panel -- mirrors
// XuiPanelSummary exactly, for the "which DNS panel should this account
// live on" picker a reseller is allowed to see.
type DNSPanelSummary struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

// DNSTemplateOption is one entry in the "pick a plan" dropdown returned by
// TestConnection/ListTemplates -- id is what gets stored as an account's
// TemplateID.
type DNSTemplateOption struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

type TestDNSConnectionResponse struct {
	Templates []DNSTemplateOption `json:"templates"`
}
