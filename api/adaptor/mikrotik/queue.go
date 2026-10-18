package mikrotik

import (
	"context"

	"github.com/maahdima/mwp/api/common"
)

type Queue struct {
	ID       string  `json:".id,omitempty"`
	Disabled string  `json:"disabled,omitempty"`
	Comment  *string `json:"comment,omitempty"`
	Name     string  `json:"name,omitempty"`
	Target   *string `json:"target,omitempty"`
	MaxLimit *string `json:"max-limit,omitempty"`
}

func (s *ServerAdaptor) CreateSimpleQueue(c context.Context, queue Queue) (*Queue, error) {
	var createdQueue Queue

	httpClient := s.client()

	err := httpClient.Put(
		c,
		common.QueuePath,
		queue,
		&createdQueue,
	)
	if err != nil {
		return nil, err
	}

	return &createdQueue, nil
}

func (s *ServerAdaptor) UpdateSimpleQueue(c context.Context, queueID string, queue Queue) (*Queue, error) {
	var updatedQueue Queue

	httpClient := s.client()

	err := httpClient.Patch(
		c,
		common.QueuePath+"/"+queueID,
		queue,
		&updatedQueue,
	)
	if err != nil {
		return nil, err
	}

	return &updatedQueue, nil
}

func (s *ServerAdaptor) DeleteSimpleQueue(c context.Context, queueID string) error {
	httpClient := s.client()

	err := httpClient.Delete(
		c,
		common.QueuePath+"/"+queueID,
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}
