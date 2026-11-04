// Package reniec defines a provisional identity interface with a synthetic provider only.
// It does not implement an official RENIEC endpoint or verify a person's identity.
package reniec

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("synthetic identity not found")

type IdentityService interface {
	Lookup(context.Context, IdentityQuery) (Identity, error)
}

type MockIdentityService struct{ records map[string]Identity }

func NewMockIdentityService(records []Identity) (*MockIdentityService, error) {
	s := &MockIdentityService{records: make(map[string]Identity, len(records))}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return nil, err
		}
		if _, exists := s.records[record.DNI]; exists {
			return nil, errors.New("duplicate synthetic identity")
		}
		s.records[record.DNI] = record
	}
	return s, nil
}

func (s *MockIdentityService) Lookup(ctx context.Context, query IdentityQuery) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	if err := query.Validate(); err != nil {
		return Identity{}, err
	}
	record, exists := s.records[query.DNI]
	if !exists {
		return Identity{}, ErrNotFound
	}
	return record, nil
}
