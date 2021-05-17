package mockjson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NectGmbH/dns"
)

// Provider is a mocked dns provider which runs in mem
type Provider struct {
	state   []dns.Zone
	lock    sync.Mutex
	counter int64
	path    string
}

// NewProvider creates a new mocked dns provider with (optionally) preexisting dns zones
func NewProvider(seed []dns.Zone, path string) *Provider {
	var provider = Provider{state: seed,
		path: path}
	provider.save()
	return &provider
}

// CreateZone creates the passed zone
func (p *Provider) CreateZone(zone dns.Zone) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.counter++

	p.state = append(p.state, zone)
	p.save()
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
	p.counter++
	for i, z := range p.state {
		if z.Name == zone.Name {
			p.state[i] = zone
			p.save()
			return nil
		}
	}
	p.save()
	return fmt.Errorf("couldn't update zone `%s` since it doesnt exist", zone.Name)
}

// DeleteZone deletes the passed zone from the provider.
func (p *Provider) DeleteZone(name string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.counter++
	for i, z := range p.state {
		if z.Name == name {
			p.state = append(p.state[:i], p.state[i+1:]...)
			p.save()
			return nil
		}
	}
	p.save()
	return fmt.Errorf("zone `%s` couldn't be deleted since it didnt exist", name)
}

func randomString() string {
	rand.Seed(time.Now().Unix())

	//Only lowercase
	charSet := "abcdedfghijklmnopqrst"
	var output strings.Builder
	length := 10
	for i := 0; i < length; i++ {
		random := rand.Intn(len(charSet))
		randomChar := charSet[random]
		output.WriteString(string(randomChar))
	}
	return output.String()
}

// ProviderOutput is the json output structure
type ProviderOutput struct {
	State   *[]dns.Zone
	Counter int64
}

// DeleteZone deletes the passed zone from the provider.
func (p *Provider) save() error {

	bytes, err := json.Marshal(ProviderOutput{
		State:   &p.state,
		Counter: p.counter,
	})

	if err != nil {
		return err
	}
	var pathTmp = p.path + randomString()
	err = ioutil.WriteFile(pathTmp, bytes, 0644)
	err = os.Rename(pathTmp, p.path)
	return nil
}
