package mikrotik

import (
	"context"
	"fmt"

	"github.com/maahdima/mwp/api/common"
)

// Container mirrors RouterOS 7's REST /container resource -- the fields
// this codebase actually reads back from a real RouterOS device's own
// JSON field names (kebab-case, confirmed against RouterOS 7's container
// REST API). Status is one of "running" | "stopped" | "stopping" |
// "starting" | "error" (RouterOS's own values, not this codebase's).
type Container struct {
	ID      string `json:".id,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Image   string `json:"tag,omitempty"`
	RootDir string `json:"root-dir,omitempty"`
}

// FetchAllContainers lists every RouterOS container on the given server --
// serverName is forwarded straight to MwpClients.GetClient rather than
// passed as nil (the convention every OTHER adaptor method in this package
// currently follows), because a stopped-container diagnosis is only
// meaningful against the SPECIFIC router an XuiPanel is mapped to
// (XuiPanel.ContainerServerID) -- silently querying "whichever client the
// map happens to return first" (GetClient(nil)'s own documented behavior)
// would misreport a panel's container status whenever more than one
// RouterOS server is registered. See TunnelGraphService.Discover's own doc
// comment for this exact pre-existing multi-router limitation elsewhere in
// this codebase; this method deliberately does not repeat it.
func (a *Adaptor) FetchAllContainers(c context.Context, serverName string) ([]Container, error) {
	var containers []Container

	httpClient := a.mwpClients.GetClient(&serverName)
	if httpClient == nil {
		return nil, fmt.Errorf("no mikrotik client configured for server %q", serverName)
	}

	err := httpClient.Get(
		c,
		common.ContainerPath,
		&containers,
	)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// FetchContainerByName finds a single container by its RouterOS-assigned
// Name on the given server -- used by the panel-container health check
// (V2RayPanelHealthService) to answer "is THIS specific container, the one
// mapped to this XuiPanel, actually running right now." Returns an error
// if the container isn't found (a container renamed/removed on the router
// outside this panel's knowledge is a real, reportable misconfiguration,
// not a silent no-op).
func (a *Adaptor) FetchContainerByName(c context.Context, serverName, containerName string) (*Container, error) {
	containers, err := a.FetchAllContainers(c, serverName)
	if err != nil {
		return nil, err
	}

	for i := range containers {
		if containers[i].Name == containerName {
			return &containers[i], nil
		}
	}

	return nil, fmt.Errorf("container %q not found on server %q", containerName, serverName)
}
