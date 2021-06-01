package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NectGmbH/dns"

	"github.com/sirupsen/logrus"
)

// DNSController represents a controller which checks the upstream (e.g. autodns) configuration, compares it with the wanted state and does changes
type DNSController struct {
	metrics                        *Metrics
	currentZonesEnforceSync        bool
	currentZones                   dns.Zones
	currentZonesHash               uint64
	currentZonesHashMap            map[string]uint64
	currentZoneNames               []string
	wantedZones                    dns.Zones
	wantedZonesHash                uint64
	wantedZonesLBHash              uint64
	wantedLBs                      *LoadbalancerList
	wantedLBsCandidate             *LoadbalancerList
	wantedLBsLock                  sync.RWMutex
	lastCurrentZoneSync            time.Time
	currentZoneSyncEnforceInterval time.Duration
	syncInterval                   time.Duration
	stopCh                         chan struct{}
	lbUpdateLoopStopCh             chan struct{}
	running                        bool
	lastCycle                      chan time.Time

	dnsProvider dns.Provider
	updates     chan *LoadbalancerList
}

// NewDNSController creates a new controller
func NewDNSController(dnsProvider dns.Provider, updates chan *LoadbalancerList, metrics *Metrics, lastCycle chan time.Time, syncInterval, currentZoneSyncEnforceInterval time.Duration) *DNSController {
	ctrl := &DNSController{
		dnsProvider:                    dnsProvider,
		updates:                        updates,
		wantedLBsLock:                  sync.RWMutex{},
		currentZoneSyncEnforceInterval: currentZoneSyncEnforceInterval,
		syncInterval:                   syncInterval,
		metrics:                        metrics,
		lastCycle:                      lastCycle,
	}
	return ctrl
}

func (c *DNSController) updateCurrentZones() error {
	logrus.Info("retrieving current zones from upstream")
	defer logrus.Info("finished retrieving current zones from upstream")

	if c.wantedLBs == nil {
		return fmt.Errorf("can't update current zones, since we're still missing list of wanted loadbalancers")
	}

	allZoneNames, err := c.dnsProvider.ListZones()
	if err != nil {
		return fmt.Errorf("couldn't list dns zones, see: %v", err)
	}

	// sort zone names by length desc so that foo.bar.org takes priority over bar.org
	// for matching lb baz.foo.bar.org
	sort.Slice(allZoneNames, func(i, j int) bool {
		return len(allZoneNames[i]) > len(allZoneNames[j])
	})

	// Find all zones from upstream we care about
	zoneNames := make(map[string]struct{})
	for _, lb := range c.wantedLBs.Loadbalancers {
		for _, zone := range allZoneNames {
			if strings.Contains(lb.Name, zone) {
				zoneNames[zone] = struct{}{}
				break
			}
		}
	}

	logrus.Debugf("updateCurrentZones: all zones we know: %+v zones we care about: %+v", allZoneNames, zoneNames)

	currentZones := make(dns.Zones, 0)

	for zoneName := range zoneNames {
		zone, err := c.dnsProvider.GetZone(zoneName)
		if err != nil {
			return fmt.Errorf("couldn't retrieve information for zone `%s`, see: %v", zoneName, err)
		}

		currentZones = append(currentZones, &zone)
	}

	c.currentZones = currentZones
	c.currentZoneNames = allZoneNames
	c.currentZonesHash = currentZones.Hash()
	logrus.Debugf("updateCurrentZones: updated current zones to %+v (Hash: %d)", c.currentZones, c.currentZonesHash)

	c.currentZonesHashMap = make(map[string]uint64)
	for _, z := range currentZones {
		c.currentZonesHashMap[z.Name] = z.Hash()
	}

	c.currentZonesEnforceSync = false
	c.lastCurrentZoneSync = time.Now()

	return nil
}

func (c *DNSController) mapLBsToZones(lbs *LoadbalancerList, zoneNames []string) map[string][]Loadbalancer {
	lbMap := make(map[string][]Loadbalancer)

	for _, lb := range lbs.Loadbalancers {
		(func(lb Loadbalancer) {
			for _, zone := range zoneNames {
				if strings.Contains(lb.Name, zone) {
					lbMap[zone] = append(lbMap[zone], lb)
					break
				}
			}
		})(lb)
	}

	return lbMap
}

func (c *DNSController) updateWantedZones() error {
	wanted := c.currentZones.Copy()
	lbMap := c.mapLBsToZones(c.wantedLBs, c.currentZoneNames)

	for _, zone := range wanted {
		lbs := lbMap[zone.Name]
		if len(lbs) == 0 {
			logrus.Debugf("updateWantedZones: skipping zone `%s` since no lbs are configured", zone.Name)
			continue
		}

		// First we get rid of all current A records in the zone with the same name as our lbs
		for _, lb := range lbs {
			aName := strings.TrimSuffix(lb.Name, "."+zone.Name)
			zone.RemoveRecordsWithName(aName)
		}

		// Now we can re-add all endpoints
		for _, lb := range lbs {
			aName := strings.TrimSuffix(lb.Name, "."+zone.Name)

			for _, endpoint := range lb.Endpoints {
				rec := dns.Record{
					Class:      dns.ClassIN,
					Name:       aName,
					TTL:        60,
					Type:       dns.RecordTypeA,
					Value:      endpoint.IP.String(),
					Preference: 0,
				}

				zone.Records = append(zone.Records, rec)
				logrus.Debugf("updateWantedZones: added a record `%s` to `%s` for lb `%s` to zone `%s`", aName, endpoint.IP.String(), lb.Name, zone.Name)
			}
		}
	}

	c.wantedZones = wanted
	c.wantedZonesHash = wanted.Hash()
	c.wantedZonesLBHash = c.wantedLBs.Hash

	return nil
}

func (c *DNSController) lbUpdateLoop() {
	for {
		select {
		case newLBs := <-c.updates:
			logrus.Debugf("WantedLBsCandidate: %+v", newLBs)

			if c.wantedLBs != nil && newLBs.Hash == c.wantedLBs.Hash {
				logrus.Debug("ignoring new wantedLBsCandidate since its the same as the current")
				continue
			}

			if c.metrics != nil {
				c.metrics.LBUpdatesTotal.Inc()
			}

			logrus.Debugf("received new wantedLBsCandidate (%d)", newLBs.Hash)
			c.wantedLBsLock.Lock()
			c.wantedLBsCandidate = newLBs
			c.wantedLBsLock.Unlock()

		case <-c.lbUpdateLoopStopCh:
			close(c.lbUpdateLoopStopCh)
			return
		}
	}

}

func (c *DNSController) sync(force bool) {
	c.lastCycle <- time.Now()

	// make sure that we get at least every n minutes the newest zone informations from upstream,
	// in case something gets fucked up there
	timeSinceLastCurrentZoneSync := time.Since(c.lastCurrentZoneSync)
	if force || c.currentZonesEnforceSync || timeSinceLastCurrentZoneSync > c.currentZoneSyncEnforceInterval {
		err := c.updateCurrentZones()
		if err != nil {
			if c.metrics != nil {
				c.metrics.ErrorsTotal.Inc()
				c.metrics.ErrorsDNS.Inc()
			}

			logrus.WithFields(logrus.Fields{
				"detail":                       err,
				"timeSinceLastCurrentZoneSync": timeSinceLastCurrentZoneSync.String(),
				"enforced":                     c.currentZonesEnforceSync,
			}).Errorf("couldn't enforce sync of current zones")
			return
		}
	}

	// Only regenerate the wanted zones, when the lbs got changed
	if force || c.wantedZonesLBHash != c.wantedLBs.Hash {
		logrus.Debugf("regenerating wanted zones since lb hashes mismatch (new: %d old: %d)", c.wantedLBs.Hash, c.wantedZonesLBHash)

		err := c.updateWantedZones()
		if err != nil {
			if c.metrics != nil {
				c.metrics.ErrorsTotal.Inc()
				c.metrics.ErrorsDNS.Inc()
			}

			logrus.WithFields(logrus.Fields{
				"detail": err,
			}).Errorf("couldnt sync wanted zones")
			return
		}
	} else {
		logrus.Debugf("not regenerating wanted zones since wantedZonesLbHash == wantedLBsHash")
	}

	// Skip further synchronisation if the world already reflects our state.
	if !force && c.currentZonesHash == c.wantedZonesHash {
		logrus.Debugf("skipping sync, current zone hash %d == wanted zone hash %d", c.currentZonesHash, c.wantedZonesHash)
		logrus.Debugf("Current: %+v Wanted: %+v", c.currentZones, c.wantedZones)
		return
	}

	// So, we now that something changed, lets try to update the individual zones
	for _, z := range c.wantedZones {
		curHash := c.currentZonesHashMap[z.Name]
		newHash := z.Hash()

		if curHash != newHash {
			logrus.Debugf("updating zone `%v` since it changed (cur: %d new: %d)", z, curHash, newHash)
			err := c.dnsProvider.UpdateZone(*z)
			if err != nil {
				if c.metrics != nil {
					c.metrics.ErrorsTotal.Inc()
					c.metrics.ErrorsDNS.Inc()
				}

				logrus.Errorf("couldn't update zone `%s`, see: %v", z.Name, err)
			} else {
				if c.metrics != nil {
					c.metrics.DNSZoneUpdates.WithLabelValues(z.Name).Inc()
				}

				logrus.Infof("updated zone `%s`", z.Name)
				c.currentZonesEnforceSync = true
			}
		} else {
			logrus.Debugf("skipping zone `%s` since it hasnt changed", z.Name)
		}
	}
}

// Run starts the controller in a async, non blocking way
func (c *DNSController) Run() {
	if c.running {
		return
	}

	c.running = true

	c.stopCh = make(chan struct{})
	c.lbUpdateLoopStopCh = make(chan struct{})

	go c.lbUpdateLoop()

	go (func() {
		for {
			select {
			case <-c.stopCh:
				close(c.stopCh)
				logrus.Infof("dns controller stopped.")
				return
			case <-time.After(c.syncInterval):
				force := false

				// sync wanted lbs if needed so we can access it in a threadsafe way
				if c.wantedLBsCandidate != nil {
					c.wantedLBsLock.Lock()
					logrus.Debugf("updating wantedLBs to candidate (%d)...", c.wantedLBsCandidate.Hash)

					if c.wantedLBsCandidate != nil {
						c.wantedLBs = c.wantedLBsCandidate
						c.wantedLBsCandidate = nil
						force = true
					}

					logrus.Debugf("updated wantedLBs to candidate (%d).", c.wantedLBs.Hash)
					c.wantedLBsLock.Unlock()
				}

				if c.wantedLBs == nil {
					logrus.Infof("still waiting for wantedLBs to get synchronized from etcd")
					continue
				}

				if force {
					logrus.Infof("updated loadbalancers, enforcing full dns sync")
				}

				now := time.Now()
				logrus.Debugf("started dns sync")
				c.sync(force)

				duration := time.Since(now)
				logrus.Debugf("finished dns sync, it took %s", duration.String())

				if c.metrics != nil {
					c.metrics.DNSSyncTime.Observe(duration.Seconds() * 1e3)
				}
			}
		}
	})()
}

// Stop stops the controller
func (c *DNSController) Stop() {
	if !c.running {
		return
	}

	c.stopCh <- struct{}{}
	c.lbUpdateLoopStopCh <- struct{}{}

	c.running = false
}
