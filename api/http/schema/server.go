package schema

type ServerStatus string

var (
	AvailableServer    ServerStatus = "available"
	NotAvailableServer ServerStatus = "not_available"
)

type CreateServerRequest struct {
	Comment   *string `json:"comment,omitempty"`
	Name      string  `json:"name" validate:"required"`
	IPAddress string  `json:"ip_address" validate:"required"`
	APIPort   string  `json:"api_port" validate:"required"`
	IsSSL     *bool   `json:"is_ssl" validate:"required"`
	Username  string  `json:"username" validate:"required"`
	Password  string  `json:"password" validate:"required"`
}

type UpdateServerRequest struct {
	Comment   *string `json:"comment,omitempty"`
	Name      *string `json:"name,omitempty"`
	IPAddress *string `json:"ip_address,omitempty"`
	APIPort   *string `json:"api_port,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
}

type ServerResponse struct {
	Id        uint         `json:"id"`
	Comment   *string      `json:"comment"`
	Name      string       `json:"name"`
	IPAddress string       `json:"ip_address"`
	APIPort   string       `json:"api_port"`
	IsActive  bool         `json:"is_active"`
	Status    ServerStatus `json:"status"`
}

type ServerStatsResponse struct {
	TotalServers  int `json:"total_servers"`
	ActiveServers int `json:"active_servers"`
}

// ServerEndpointResponse exposes only the connection info a WireGuard peer
// config needs. Unlike ServerResponse it carries no credentials and is safe
// to return to any authenticated user, including resellers, who otherwise
// have no access to server management.
type ServerEndpointResponse struct {
	Id        uint   `json:"id"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
}

// ServerConnectionHealthResponse is GetConnectionHealth's admin-only, live
// diagnostic result. Reason is a short machine-readable code -- "ok",
// "timeout", "connection_refused", "unauthorized", "bad_status",
// "not_configured", "db_error", "request_error", or "connection_error" --
// and Detail is a human-readable elaboration (latency on success; the
// underlying error/HTTP status on failure).
type ServerConnectionHealthResponse struct {
	Connected bool   `json:"connected"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail"`
}
