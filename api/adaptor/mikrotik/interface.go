package mikrotik

import (
	"context"
	"fmt"

	"github.com/maahdima/mwp/api/common"
)

type Interface struct {
	ID        string `json:".id,omitempty"`
	Name      string `json:"name,omitempty"`
	ActualMTU string `json:"actual-mtu,omitempty"`
	Disabled  string `json:"disabled,omitempty"`
	MTU       string `json:"mtu,omitempty"`
	TxByte    string `json:"tx-byte,omitempty"`
	RxByte    string `json:"rx-byte,omitempty"`
	Running   string `json:"running,omitempty"`
	Type      string `json:"type,omitempty"`
}

func (a *Adaptor) FetchInterface(c context.Context, interfaceID string) (*Interface, error) {
	var iface Interface

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.InterfacePath+"/"+interfaceID,
		&iface,
	)
	if err != nil {
		return nil, err
	}

	return &iface, nil
}

// FetchAllInterfaces lists every interface on the router regardless of
// type (ether/wireguard/gre/ipip/eoip/bridge/etc) via the generic
// /interface path -- used by TunnelHealthService to poll whatever
// TunnelGraphService's discovery pass flagged as an infrastructure
// tunnel, without needing a separate typed fetch call per possible
// tunnel technology.
func (a *Adaptor) FetchAllInterfaces(c context.Context) ([]Interface, error) {
	var ifaces []Interface

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.InterfacePath,
		&ifaces,
	)
	if err != nil {
		return nil, err
	}

	return ifaces, nil
}

// SetInterfaceDisabledByName looks up name's own current ".id" (the
// generic /interface path doesn't support updating by name directly)
// and PATCHes its disabled flag -- the self-healing engine's Level 1
// action (spec section ب-5), works uniformly across every interface
// type since RouterOS's own disabled=yes|no applies the same way
// regardless of whether the underlying interface is ether/wireguard/
// gre/ipip/eoip.
func (a *Adaptor) SetInterfaceDisabledByName(c context.Context, name string, disabled bool) error {
	ifaces, err := a.FetchAllInterfaces(c)
	if err != nil {
		return err
	}
	var target *Interface
	for i := range ifaces {
		if ifaces[i].Name == name {
			target = &ifaces[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("interface %q not found", name)
	}

	disabledValue := "false"
	if disabled {
		disabledValue = "true"
	}

	httpClient := a.mwpClients.GetClient(nil)
	return httpClient.Patch(
		c,
		common.InterfacePath+"/"+target.ID,
		Interface{Disabled: disabledValue},
		&Interface{},
	)
}
