package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"futures-arbitrage-scanner/storage"
)

const historyFlushInterval = 250 * time.Millisecond

type pendingRoute struct {
	earliest storage.Observation
	peak     storage.Observation
	latest   storage.Observation
}

type opportunityHistory struct {
	store storage.OpportunityStore

	mu      sync.Mutex
	pending map[string]pendingRoute
	closed  bool
	wake    chan struct{}
	done    chan context.Context
	wg      sync.WaitGroup
	worker  context.Context
	cancel  context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

func newOpportunityHistory(store storage.OpportunityStore, _ int) *opportunityHistory {
	workerContext, cancel := context.WithCancel(context.Background())
	history := &opportunityHistory{
		store:   store,
		pending: make(map[string]pendingRoute),
		wake:    make(chan struct{}, 1),
		done:    make(chan context.Context, 1),
		worker:  workerContext,
		cancel:  cancel,
	}
	history.wg.Add(1)
	go history.run()
	return history
}

func observationFrom(opportunity ArbitrageOpportunity) storage.Observation {
	return storage.Observation{
		Symbol:         opportunity.Symbol,
		BuySource:      opportunity.BuySource,
		SellSource:     opportunity.SellSource,
		BuyPrice:       opportunity.BuyPrice,
		SellPrice:      opportunity.SellPrice,
		GrossSpreadPct: opportunity.ProfitPct,
		ObservedAtMS:   opportunity.Timestamp,
	}
}

func observationKey(observation storage.Observation) string {
	return fmt.Sprintf("%s\x00%s\x00%s", observation.Symbol, observation.BuySource, observation.SellSource)
}

func (h *opportunityHistory) Observe(opportunity ArbitrageOpportunity) bool {
	observation := observationFrom(opportunity)
	key := observationKey(observation)

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return false
	}
	pending, exists := h.pending[key]
	if !exists {
		pending = pendingRoute{earliest: observation, peak: observation, latest: observation}
	} else {
		pending = mergePendingRoute(pending, pendingRoute{
			earliest: observation,
			peak:     observation,
			latest:   observation,
		})
	}
	h.pending[key] = pending
	h.mu.Unlock()

	select {
	case h.wake <- struct{}{}:
	default:
	}
	return true
}

func mergePendingRoute(left, right pendingRoute) pendingRoute {
	if left.earliest.ObservedAtMS == 0 || (right.earliest.ObservedAtMS > 0 && right.earliest.ObservedAtMS < left.earliest.ObservedAtMS) {
		left.earliest = right.earliest
	}
	if right.peak.GrossSpreadPct > left.peak.GrossSpreadPct {
		left.peak = right.peak
	}
	if right.latest.ObservedAtMS >= left.latest.ObservedAtMS {
		left.latest = right.latest
	}
	return left
}

func orderedObservations(pending pendingRoute) []storage.Observation {
	ordered := make([]storage.Observation, 0, 3)
	for _, observation := range []storage.Observation{pending.earliest, pending.peak, pending.latest} {
		if observation.ObservedAtMS <= 0 {
			continue
		}
		duplicate := false
		for _, existing := range ordered {
			if existing == observation {
				duplicate = true
				break
			}
		}
		if !duplicate {
			ordered = append(ordered, observation)
		}
	}
	return ordered
}

func (h *opportunityHistory) takePending() map[string]pendingRoute {
	h.mu.Lock()
	pending := h.pending
	h.pending = make(map[string]pendingRoute)
	h.mu.Unlock()
	return pending
}

func (h *opportunityHistory) requeuePending(pending map[string]pendingRoute) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, route := range pending {
		if current, exists := h.pending[key]; exists {
			route = mergePendingRoute(route, current)
		}
		h.pending[key] = route
	}
}

func (h *opportunityHistory) persistPending(ctx context.Context) error {
	pendingRoutes := h.takePending()
	if len(pendingRoutes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(pendingRoutes))
	for key := range pendingRoutes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	observations := make([]storage.Observation, 0, len(keys)*3)
	for _, key := range keys {
		observations = append(observations, orderedObservations(pendingRoutes[key])...)
	}
	if err := h.store.ObserveBatch(ctx, observations); err != nil {
		log.Printf("Opportunity history write failed: %v", err)
		h.requeuePending(pendingRoutes)
		return err
	}
	return nil
}

func (h *opportunityHistory) drainForShutdown(ctx context.Context) {
	if err := h.persistPending(ctx); err != nil {
		h.closeErr = errors.Join(h.closeErr, fmt.Errorf("drain opportunity history: %w", err))
	}
}

func (h *opportunityHistory) waitForFlush(shutdown <-chan context.Context) (context.Context, bool) {
	timer := time.NewTimer(historyFlushInterval)
	defer timer.Stop()
	select {
	case shutdownContext := <-shutdown:
		return shutdownContext, true
	case <-timer.C:
		for {
			select {
			case <-h.wake:
			default:
				return nil, false
			}
		}
	}
}

func (h *opportunityHistory) run() {
	defer h.wg.Done()
	staleTicker := time.NewTicker(5 * time.Second)
	pruneTicker := time.NewTicker(time.Hour)
	defer staleTicker.Stop()
	defer pruneTicker.Stop()

	for {
		select {
		case shutdownContext := <-h.done:
			h.drainForShutdown(shutdownContext)
			return
		default:
		}

		select {
		case shutdownContext := <-h.done:
			h.drainForShutdown(shutdownContext)
			return
		case <-h.wake:
			if shutdownContext, shuttingDown := h.waitForFlush(h.done); shuttingDown {
				h.drainForShutdown(shutdownContext)
				return
			}
			ctx, cancel := context.WithTimeout(h.worker, 5*time.Second)
			_ = h.persistPending(ctx)
			cancel()
		case now := <-staleTicker.C:
			ctx, cancel := context.WithTimeout(h.worker, 5*time.Second)
			if err := h.store.CloseStale(ctx, now.Add(-15*time.Second).UnixMilli()); err != nil {
				log.Printf("Opportunity history stale-close failed: %v", err)
			}
			cancel()
		case now := <-pruneTicker.C:
			ctx, cancel := context.WithTimeout(h.worker, 5*time.Second)
			if err := h.store.Prune(ctx, now.Add(-7*24*time.Hour).UnixMilli()); err != nil {
				log.Printf("Opportunity history prune failed: %v", err)
			}
			cancel()
		}
	}
}

func (h *opportunityHistory) Close(ctx context.Context) error {
	h.closeOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.cancel()
		h.done <- ctx
		h.wg.Wait()
		h.closeErr = errors.Join(h.closeErr, h.store.Close())
	})
	return h.closeErr
}
