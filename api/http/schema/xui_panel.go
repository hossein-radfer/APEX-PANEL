package schema

type CreateXuiPanelRequest struct {
	Name             string `json:"name" validate:"required"`
	SaleTitle        string `json:"sale_title" validate:"required"`
	APIBaseURL       string `json:"api_base_url" validate:"required"`
	Username         string `json:"username" validate:"required"`
	Password         string `json:"password" validate:"required"`
	DefaultInboundID int    `json:"default_inbound_id" validate:"required"`
	Protocol         string `json:"protocol" validate:"required"`
	SubBaseURL       string `json:"sub_base_url" validate:"required"`

	// ContainerServerID/ContainerName (item 9) are both optional -- a
	// panel not hosted inside a RouterOS container (or one whose admin
	// hasn't configured the mapping yet) simply omits both, exactly as
	// every panel created before this feature existed already does.
	ContainerServerID *uint   `json:"container_server_id,omitempty"`
	ContainerName     *string `json:"container_name,omitempty"`
}

// TestXuiPanelConnectionRequest is the minimal field set needed to test
// connectivity BEFORE a panel has been saved -- deliberately does not
// require DefaultInboundID/Name/SaleTitle/Protocol/SubBaseURL, since
// discovering the inbound list via this exact call is what lets the admin
// fill in DefaultInboundID afterward (see XuiPanelController.
// TestConnectionUnsaved). Mirrors the frontend's TestXuiPanelSchema
// exactly.
type TestXuiPanelConnectionRequest struct {
	APIBaseURL string `json:"api_base_url" validate:"required"`
	Username   string `json:"username" validate:"required"`
	Password   string `json:"password" validate:"required"`
}

type UpdateXuiPanelRequest struct {
	Name             string  `json:"name,omitempty"`
	SaleTitle        string  `json:"sale_title,omitempty"`
	APIBaseURL       string  `json:"api_base_url,omitempty"`
	Username         string  `json:"username,omitempty"`
	Password         *string `json:"password,omitempty"` // omitted/nil leaves the existing password unchanged
	DefaultInboundID int     `json:"default_inbound_id,omitempty"`
	Protocol         string  `json:"protocol,omitempty"`
	SubBaseURL       string  `json:"sub_base_url,omitempty"`

	// ContainerServerID/ContainerName (item 9) -- omitted (nil) leaves the
	// existing mapping unchanged, matching Password's own convention.
	// ClearContainerMapping explicitly removes an existing mapping (e.g.
	// the panel was migrated off a RouterOS container onto a plain VPS) --
	// needed because a bare nil on the two fields above is ambiguous
	// between "don't touch" and "unset."
	ContainerServerID      *uint   `json:"container_server_id,omitempty"`
	ContainerName          *string `json:"container_name,omitempty"`
	ClearContainerMapping  *bool   `json:"clear_container_mapping,omitempty"`
}

type XuiPanelResponse struct {
	Id               uint    `json:"id"`
	Name             string  `json:"name"`
	SaleTitle        string  `json:"sale_title"`
	APIBaseURL       string  `json:"api_base_url"`
	Username         string  `json:"username"`
	DefaultInboundID int     `json:"default_inbound_id"`
	Protocol         string  `json:"protocol"`
	SubBaseURL       string  `json:"sub_base_url"`
	Status           string  `json:"status"`
	LastError        *string `json:"last_error,omitempty"`
	LastSyncedAt     *string `json:"last_synced_at,omitempty"`

	// ContainerServerID/ContainerServerName/ContainerName (item 9) --
	// ContainerServerName is resolved server-side (join against
	// model.Server) purely so the admin UI doesn't need a second request
	// just to show which router's name a raw ID refers to.
	ContainerServerID   *uint   `json:"container_server_id,omitempty"`
	ContainerServerName *string `json:"container_server_name,omitempty"`
	ContainerName       *string `json:"container_name,omitempty"`
}

// XuiInboundOption is one entry in the "pick an inbound" dropdown returned
// by TestConnection -- id is what gets stored as DefaultInboundID.
type XuiInboundOption struct {
	Id       int    `json:"id"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
}

// XuiPanelSummary is the minimal, reseller-safe view of a panel -- just
// enough to render a "which server(s) should this package live on"
// picker. Deliberately excludes APIBaseURL/Username/Password/etc (the
// full XuiPanelResponse), which are admin-only and never meant to reach
// a reseller session. Returned by V2RayPackageController.
// GetAssignedXuiPanels, the one panel-related endpoint a reseller is
// actually permitted to call for their own id.
type XuiPanelSummary struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type TestXuiConnectionResponse struct {
	Inbounds []XuiInboundOption `json:"inbounds"`
}
