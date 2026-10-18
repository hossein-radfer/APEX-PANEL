package mikrotik

import (
	"context"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/common"
)

type WireGuardInterface struct {
	ID         string  `json:".id,omitempty"`
	Disabled   string  `json:"disabled,omitempty"`
	Comment    *string `json:"comment,omitempty"`
	ListenPort string  `json:"listen-port,omitempty"`
	MTU        string  `json:"mtu,omitempty"`
	Name       string  `json:"name,omitempty"`
	PrivateKey string  `json:"private-key,omitempty"`
	PublicKey  string  `json:"public-key,omitempty"`
	Running    *string `json:"running,omitempty"`
}

func (s *ServerAdaptor) FetchWgInterfaces(c context.Context) ([]WireGuardInterface, error) {
	var wgInterfaces []WireGuardInterface

	httpClient := s.client()

	err := httpClient.Get(
		c,
		common.WGInterfacePath,
		&wgInterfaces,
	)
	if err != nil {
		return nil, err
	}

	return wgInterfaces, nil
}

func (s *ServerAdaptor) FetchWgInterface(c context.Context, interfaceID string) (*WireGuardInterface, error) {
	var wgInterface WireGuardInterface

	httpClient := s.client()

	err := httpClient.Get(
		c,
		common.WGInterfacePath+"/"+interfaceID,
		&wgInterface,
	)
	if err != nil {
		return nil, err
	}

	return &wgInterface, nil
}

func (s *ServerAdaptor) CreateWgInterface(c context.Context, wgInterface WireGuardInterface) (*WireGuardInterface, error) {
	var createdInterface WireGuardInterface

	httpClient := s.client()

	err := httpClient.Put(
		c,
		common.WGInterfacePath,
		wgInterface,
		&createdInterface,
	)
	if err != nil {
		return nil, err
	}

	return &createdInterface, nil
}

func (s *ServerAdaptor) UpdateWgInterface(c context.Context, interfaceID string, wgInterface WireGuardInterface) (*WireGuardInterface, error) {
	var updatedInterface WireGuardInterface

	httpClient := s.client()

	err := httpClient.Patch(
		c,
		common.WGInterfacePath+"/"+interfaceID,
		wgInterface,
		&updatedInterface,
	)
	if err != nil {
		return nil, err
	}

	return &updatedInterface, nil
}

func (s *ServerAdaptor) DeleteWgInterface(c context.Context, interfaceID string) error {
	httpClient := s.client()

	err := httpClient.Delete(
		c,
		common.WGInterfacePath+"/"+interfaceID,
		nil,
	)
	if err != nil {
		s.adaptor.logger.Error("failed to delete wireguard peer", zap.Error(err))
		return err
	}

	return nil
}
