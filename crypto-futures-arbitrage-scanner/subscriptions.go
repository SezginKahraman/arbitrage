package main

import (
	"context"
	"slices"
	"sort"
	"sync"
)

type sourceSubscriptionRunner func(context.Context, []string)

type activeSubscription struct {
	symbols []string
	cancel  context.CancelFunc
}

type subscriptionSupervisor struct {
	mutex   sync.Mutex
	runners map[string]sourceSubscriptionRunner
	active  map[string]activeSubscription
	stopped bool
}

func newSubscriptionSupervisor(runners map[string]sourceSubscriptionRunner) *subscriptionSupervisor {
	cloned := make(map[string]sourceSubscriptionRunner, len(runners))
	for source, runner := range runners {
		cloned[source] = runner
	}
	return &subscriptionSupervisor{runners: cloned, active: make(map[string]activeSubscription)}
}

func normalizedSubscriptionSymbols(symbols []string) []string {
	result := normalizeDiscoveredSymbols(symbols)
	sort.Strings(result)
	return result
}

func (s *subscriptionSupervisor) Reconcile(symbolsBySource map[string][]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.stopped {
		return
	}
	for source, runner := range s.runners {
		next := normalizedSubscriptionSymbols(symbolsBySource[source])
		current, exists := s.active[source]
		if exists && slices.Equal(current.symbols, next) {
			continue
		}
		if exists {
			current.cancel()
			delete(s.active, source)
		}
		if len(next) == 0 {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.active[source] = activeSubscription{symbols: append([]string(nil), next...), cancel: cancel}
		go runner(ctx, append([]string(nil), next...))
	}
}

func (s *subscriptionSupervisor) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	for source, active := range s.active {
		active.cancel()
		delete(s.active, source)
	}
}
