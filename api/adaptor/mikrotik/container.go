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

// FetchAllContainers lists every RouterOS container on this ServerAdaptor's
// own server -- this was historically the one place in this package that
// already took an explicit server identifier (serverName) rather than
// relying on GetClient(nil)'s "whichever client the map returns first"
// fallback, since a stopped-container diagnosis is only ever meaningful
// against the SPECIFIC router an XuiPanel is mapped to
// (XuiPanel.ContainerServerID). Now that every adaptor method in this
// package goes through ServerAdaptor the same way, this is no longer a
// special case -- see Adaptor.ForServer's own doc comment.
func (s *ServerAdaptor) FetchAllContainers(c context.Context) ([]Container, error) {
	var containers []Container

	httpClient := s.client()
	if httpClient == nil {
		return nil, fmt.Errorf("no mikrotik client configured for server id %d", s.serverID)
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
// Name on this ServerAdaptor's own server -- used by the panel-container
// health check (V2RayPanelHealthService) to answer "is THIS specific
// container, the one mapped to this XuiPanel, actually running right now."
// Returns an error if the container isn't found (a container renamed/
// removed on the router outside this panel's knowledge is a real,
// reportable misconfiguration, not a silent no-op).
func (s *ServerAdaptor) FetchContainerByName(c context.Context, containerName string) (*Container, error) {
	containers, err := s.FetchAllContainers(c)
	if err != nil {
		return nil, err
	}

	for i := range containers {
		if containers[i].Name == containerName {
			return &containers[i], nil
		}
	}

	return nil, fmt.Errorf("container %q not found on server id %d", containerName, s.serverID)
}
