package metering

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrLimitExceeded = errors.New("api request limit exceeded")
	ErrEmptyEndpoint = errors.New("endpoint cannot be empty")
)

type Service struct {
	limit    uint64
	total    atomic.Uint64
	counters sync.Map // map[string]*atomic.Uint64
}

func NewService(limit uint64) *Service {
	return &Service{limit: limit}
}

func (s *Service) Increment(endpoint string) error {
	if endpoint == "" {
		return ErrEmptyEndpoint
	}

	for {
		current := s.total.Load()
		if current >= s.limit {
			return ErrLimitExceeded
		}

		if s.total.CompareAndSwap(current, current+1) {
			break
		}
	}

	counter := s.counterFor(endpoint)
	counter.Add(1)
	return nil
}

func (s *Service) Snapshot() map[string]uint64 {
	out := make(map[string]uint64)
	s.counters.Range(func(key, value any) bool {
		out[key.(string)] = value.(*atomic.Uint64).Load()
		return true
	})
	return out
}

func (s *Service) Total() uint64 {
	return s.total.Load()
}

func (s *Service) Limit() uint64 {
	return s.limit
}

func (s *Service) counterFor(endpoint string) *atomic.Uint64 {
	if existing, ok := s.counters.Load(endpoint); ok {
		return existing.(*atomic.Uint64)
	}

	counter := &atomic.Uint64{}
	actual, _ := s.counters.LoadOrStore(endpoint, counter)
	return actual.(*atomic.Uint64)
}
