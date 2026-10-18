package mikrotik

import (
	"context"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/common"
)

type WireGuardPeer struct {
	ID                     string  `json:".id,omitempty"`
	Disabled               string  `json:"disabled,omitempty"`
	Comment                *string `json:"comment,omitempty"`
	AllowedAddress         string  `json:"allowed-address,omitempty"`
	PersistentKeepAlive    *string `json:"persistent-keepalive,omitempty"`
	Interface              string  `json:"interface,omitempty"`
	Name                   string  `json:"name,omitempty"`
	PresharedKey           *string `json:"preshared-key,omitempty"`
	PrivateKey             *string `json:"private-key,omitempty"`
	PublicKey              string  `json:"public-key,omitempty"`
	ClientEndpoint         *string `json:"client-endpoint,omitempty"`
	CurrentEndpointAddress *string `json:"current-endpoint-address,omitempty"`
	CurrentEndpointPort    *string `json:"current-endpoint-port,omitempty"`
	LastHandshake          *string `json:"last-handshake,omitempty"`
	TransferRx             string  `json:"rx,omitempty"`
	TransferTx             string  `json:"tx,omitempty"`
}

func (s *ServerAdaptor) FetchWgPeers(c context.Context) ([]WireGuardPeer, error) {
	var wgPeers []WireGuardPeer

	httpClient := s.client()

	err := httpClient.Get(
		c,
		common.WGPeerPath,
		&wgPeers,
	)
	if err != nil {
		s.adaptor.logger.Error("failed to get wireguard peers", zap.Error(err))
		return nil, err
	}

	return wgPeers, nil
}

// FetchWgPeer's own failure is deliberately NOT logged here -- a confirmed,
// reported log-flood incident this fixes: httpClient.Get's underlying
// do_request.go already logs the HTTP-level details (status code, response
// body) on every non-2xx response, and every caller of this function logs
// its own higher-level failure WITH the peer ID (the one piece of context
// this function itself never had, since the error message doesn't include
// it). Logging a third, contextless "failed to get wireguard peer" here on
// top of both added no information, only volume -- for a handful of
// permanently RouterOS-side-deleted peers still present in this panel's own
// DB, that meant 3 log lines every single traffic-calculation tick,
// indefinitely, until the panel's own disk filled up.
func (s *ServerAdaptor) FetchWgPeer(c context.Context, peerID string) (*WireGuardPeer, error) {
	var wgPeer WireGuardPeer

	httpClient := s.client()

	err := httpClient.Get(
		c,
		common.WGPeerPath+"/"+peerID,
		&wgPeer,
	)
	if err != nil {
		return nil, err
	}

	return &wgPeer, nil
}

func (s *ServerAdaptor) CreateWgPeer(c context.Context, wgPeer WireGuardPeer) (*WireGuardPeer, error) {
	var createdPeer WireGuardPeer

	httpClient := s.client()

	err := httpClient.Put(
		c,
		common.WGPeerPath,
		wgPeer,
		&createdPeer,
	)
	if err != nil {
		return nil, err
	}

	return &createdPeer, nil
}

func (s *ServerAdaptor) UpdateWgPeer(c context.Context, peerID string, wgPeer WireGuardPeer) (*WireGuardPeer, error) {
	var updatedPeer WireGuardPeer

	httpClient := s.client()

	err := httpClient.Patch(
		c,
		common.WGPeerPath+"/"+peerID,
		wgPeer,
		&updatedPeer,
	)
	if err != nil {
		return nil, err
	}

	return &updatedPeer, nil
}

func (s *ServerAdaptor) DeleteWgPeer(c context.Context, peerID string) error {
	httpClient := s.client()

	err := httpClient.Delete(
		c,
		common.WGPeerPath+"/"+peerID,
		nil,
	)
	if err != nil {
		s.adaptor.logger.Error("failed to delete wireguard peer", zap.Error(err))
		return err
	}

	return nil
}
