package storage

import (
	"errors"
	"sync/atomic"
)

var ErrLimitExceeded = errors.New("storage limit exceeded")

type Service struct {
	limit uint64
	used  atomic.Uint64
}

func NewService(limit uint64) *Service {
	return &Service{limit: limit}
}

func (s *Service) TryAdd(size uint64) error {
	for {
		current := s.used.Load()
		if current > s.limit {
			return ErrLimitExceeded
		}
		if size > s.limit-current {
			return ErrLimitExceeded
		}

		if s.used.CompareAndSwap(current, current+size) {
			return nil
		}
	}
}

func (s *Service) Used() uint64 {
	return s.used.Load()
}

func (s *Service) Limit() uint64 {
	return s.limit
}
