package schema

type InterfaceResponse struct {
	Id          uint    `json:"id"`
	ServerID    uint    `json:"server_id"`
	ServerName  string  `json:"server_name"`
	InterfaceID string  `json:"interface_id"`
	Disabled    bool    `json:"disabled"`
	Comment     *string `json:"comment"`
	Name        string  `json:"name"`
	ListenPort  string  `json:"listen_port"`
	MTU         string  `json:"mtu"`
	IsRunning   bool    `json:"is_running"`
}

type CreateInterfaceRequest struct {
	// ServerID picks which Mikrotik server this interface is created on.
	// Optional while only a single-server install is common -- nil falls
	// back to that one server (see WgInterface.CreateInterface); a
	// multi-server install's UI always sends it explicitly once more than
	// one server is registered.
	ServerID   *uint   `json:"server_id,omitempty"`
	Comment    *string `json:"comment,omitempty"`
	Name       string  `json:"name" validate:"required"`
	ListenPort string  `json:"listen_port" validate:"required"`
}

type UpdateInterfaceRequest struct {
	Disabled *bool   `json:"disabled,omitempty"`
	Comment  *string `json:"comment,omitempty"`
	Name     string  `json:"name,omitempty"`
}

type InterfaceStatsResponse struct {
	TotalInterfaces  int `json:"total_interfaces"`
	ActiveInterfaces int `json:"active_interfaces"`
}
