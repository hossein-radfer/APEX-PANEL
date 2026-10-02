package schema

type AuditLogResponse struct {
	ID           uint   `json:"id"`
	ResellerID   uint   `json:"reseller_id"`
	ResellerName string `json:"reseller_name"`
	Action       string `json:"action"`
	Description  string `json:"description"`
	CreatedAt    int64  `json:"created_at"`
}
