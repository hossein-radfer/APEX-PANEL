package mikrotik

import (
	"context"
	"strconv"

	"github.com/maahdima/mwp/api/common"
)

// TorchFlow models one row of RouterOS's `/tool/torch` output -- field
// names/JSON keys follow RouterOS's own REST API convention (kebab-case,
// matching every other adaptor struct in this package), and correspond
// directly to the CLI column names the admin's own sample output uses
// (SRC-ADDRESS, IP-PROTOCOL, SRC-PORT, TX-PACKETS, RX-PACKETS).
type TorchFlow struct {
	SrcAddress    string `json:"src-address,omitempty"`
	DstAddress    string `json:"dst-address,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	SrcPort       string `json:"src-port,omitempty"`
	DstPort       string `json:"dst-port,omitempty"`
	TxRate        string `json:"tx-rate,omitempty"`
	RxRate        string `json:"rx-rate,omitempty"`
	TxPacketsRate string `json:"tx-packets-rate,omitempty"`
	RxPacketsRate string `json:"rx-packets-rate,omitempty"`
}

// FetchTorchFlows runs a single bounded-duration `/tool/torch` sample
// against ifaceName -- durationSeconds controls how long RouterOS itself
// samples before returning (NOT this call's own network timeout; keep this
// short, see ToolTorchPath's own doc comment for why). Unlike the CLI
// command, this REST call blocks for roughly durationSeconds then returns
// one bounded JSON array -- it does not keep streaming after that.
func (a *Adaptor) FetchTorchFlows(c context.Context, ifaceName string, durationSeconds int) ([]TorchFlow, error) {
	var flows []TorchFlow

	httpClient := a.mwpClients.GetClient(nil)

	reqBody := map[string]string{
		"interface": ifaceName,
		"duration":  strconv.Itoa(durationSeconds),
	}

	err := httpClient.Post(
		c,
		common.ToolTorchPath,
		reqBody,
		&flows,
	)
	if err != nil {
		return nil, err
	}

	return flows, nil
}
