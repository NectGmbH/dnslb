package mock

import (
	"fmt"
	"sync"

	"github.com/NectGmbH/dns"
)

// Provider is a mocked dns provider which runs in mem
type Provider struct {
	state []dns.Zone
	lock  sync.Mutex
}

// NewProvider creates a new mocked dns provider with (optionally) preexisting dns zones
func NewProvider(seed []dns.Zone) *Provider {
	return &Provider{state: seed}
}

// CreateZone creates the passed zone
func (p *Provider) CreateZone(zone dns.Zone) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state = append(p.state, zone)
	return nil
}

// GetZone retrieves the zone information for the zone with the passed name.
func (p *Provider) GetZone(name string) (dns.Zone, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, z := range p.state {
		if z.Name == name {
			return z, nil
		}
	}

	return dns.Zone{}, fmt.Errorf("zone `%s` not found", name)
}

// ListZones lists all zones from the provider.
func (p *Provider) ListZones() ([]string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	names := make([]string, len(p.state))

	for i, z := range p.state {
		names[i] = z.Name
	}

	return names, nil
}

// UpdateZone updates the zone on the provider.
func (p *Provider) UpdateZone(zone dns.Zone) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for i, z := range p.state {
		if z.Name == zone.Name {
			p.state[i] = zone
			return nil
		}
	}

	return fmt.Errorf("couldn't update zone `%s` since it doesnt exist", zone.Name)
}

// DeleteZone deletes the passed zone from the provider.
func (p *Provider) DeleteZone(name string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for i, z := range p.state {
		if z.Name == name {
			p.state = append(p.state[:i], p.state[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("zone `%s` couldn't be deleted since it didnt exist", name)
}
