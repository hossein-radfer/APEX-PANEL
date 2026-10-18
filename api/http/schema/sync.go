package schema

type SyncInterfacePreviewResponse struct {
	ID         string  `json:"id"`
	ServerID   uint    `json:"server_id"`
	ServerName string  `json:"server_name"`
	Disabled   bool    `json:"disabled"`
	Comment    *string `json:"comment"`
	Name       string  `json:"name"`
	ListenPort string  `json:"listen_port"`
	MTU        string  `json:"mtu"`
	IsRunning  bool    `json:"is_running"`
}

type SyncPeerPreviewResponse struct {
	ID             string  `json:"id"`
	ServerID       uint    `json:"server_id"`
	ServerName     string  `json:"server_name"`
	Disabled       bool    `json:"disabled"`
	Comment        *string `json:"comment"`
	Name           string  `json:"name"`
	Interface      string  `json:"interface"`
	AllowedAddress string  `json:"allowed_address"`
}

// SyncInterfacesRequest's InterfaceIDs are RouterOS ".id" values, unique
// only within one server -- ServerID pins the selection to exactly the
// server the preview list's rows came from, so a selection can never be
// (re-)applied against a different server that happens to reuse the same
// ".id" string. Required once the preview response carries ServerID too.
type SyncInterfacesRequest struct {
	ServerID     uint     `json:"server_id" validate:"required"`
	InterfaceIDs []string `json:"interface_ids"`
}

type SyncPeersRequest struct {
	ServerID uint     `json:"server_id" validate:"required"`
	PeerIDs  []string `json:"peer_ids"`
}
