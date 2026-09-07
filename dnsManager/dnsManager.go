// Package dnsmanager owns the resolver state for a single invocation.
package dnsmanager

import (
	"errors"
	"math/rand/v2"
	"strings"
	"sync"
)

// ErrUnavailable indicates that no usable resolver could be selected.
var ErrUnavailable = errors.New("no usable DNS resolver; check the configured resolver list")

// DNSServerEntry is a snapshot of a configured resolver.
type DNSServerEntry struct {
	Host   string
	Status bool
	Errors int
}

// Pool synchronizes resolver health updates from concurrent workers.
type Pool struct {
	mu      sync.RWMutex
	servers []DNSServerEntry
}

// New creates an independent pool. Blank lines are ignored.
func New(hosts []string) (*Pool, error) {
	p := &Pool{}
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if !strings.Contains(host, ":") {
			host += ":53"
		}
		p.servers = append(p.servers, DNSServerEntry{Host: host, Status: true})
	}
	if len(p.servers) == 0 {
		return nil, ErrUnavailable
	}
	return p, nil
}

// Pick retains the existing bounded random selection policy.
func (p *Pool) Pick() (DNSServerEntry, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.servers) == 0 {
		return DNSServerEntry{}, ErrUnavailable
	}
	for i := 0; i < 100; i++ {
		entry := p.servers[rand.IntN(len(p.servers))]
		if entry.Status {
			return entry, nil
		}
	}
	return DNSServerEntry{}, ErrUnavailable
}

// ReportError records a failure, retaining the existing error threshold.
func (p *Pool) ReportError(host string, limit int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.servers {
		if p.servers[i].Host == host {
			p.servers[i].Errors++
			if p.servers[i].Errors > limit {
				p.servers[i].Status = false
			}
		}
	}
}

// Snapshot returns a copy so callers cannot mutate the shared state.
func (p *Pool) Snapshot() []DNSServerEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]DNSServerEntry(nil), p.servers...)
}
