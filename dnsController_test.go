package main

import (
	"testing"
	"time"

	"github.com/NectGmbH/dns"
	dnsmock "github.com/NectGmbH/dns/provider/mock"
)

// TestAddRemoveEndpoints tests whether dns zones get updated correctly when endpoints get removed/added
func TestAddRemoveEndpoints(t *testing.T) {
	seed := []dns.Zone{
		{
			Name: "schnitzel.org",
			SOA: dns.SOA{
				MName:   "127.0.0.1",
				RName:   "hollondaise.schnitzel.org",
				Serial:  1,
				Refresh: 2,
				Retry:   3,
				Expire:  4,
				TTL:     5,
			},
		},
	}

	emptyLBs := NewLoadbalancerList([]Loadbalancer{})

	ctrl, dnsProvider, cycleCh, updatesCh := getTestDNSController(seed)

	ctrl.Run()
	defer ctrl.Stop()

	updatesCh <- emptyLBs

	waitDNSCycles(cycleCh, 2)

	// -- Test whether we dont change stuff without lbs ------------------------
	zoneNames, _ := dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err := dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 0 {
		t.Fatalf("expected 0 records but got %d", len(zone.Records))
	}

	// -------------------------------------------------------------------------

	eps1, err := TryParseEndpointProtocols("http://127.0.0.1:80,tcp://127.0.1.1:8080")
	if err != nil {
		t.Fatalf("couldn't parse endpoint, see: %v", err)
	}

	newLBs := NewLoadbalancerList([]Loadbalancer{
		NewLoadbalancer("zwiebel.schnitzel.org", eps1...),
	})

	updatesCh <- newLBs

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ = dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err = dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 2 {
		t.Fatalf("expected 2 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "127.0.0.1" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

	if zone.Records[1].Value != "127.0.1.1" ||
		zone.Records[1].Type != dns.RecordTypeA ||
		zone.Records[1].Name != "zwiebel" {
		t.Fatalf("expected second record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[1].Type, zone.Records[1].Name, zone.Records[1].Value)
	}

	// -------------------------------------------------------------------------

	eps1, err = TryParseEndpointProtocols("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("couldn't parse endpoint, see: %v", err)
	}

	newLBs = NewLoadbalancerList([]Loadbalancer{
		NewLoadbalancer("zwiebel.schnitzel.org", eps1...),
	})

	updatesCh <- newLBs

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ = dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err = dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 1 {
		t.Fatalf("expected 1 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "127.0.0.1" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

}

// TestRecreateWhenUpstreamChanges tests that zones get recreated when tempered with upstream
func TestRecreateWhenUpstreamChanges(t *testing.T) {
	seed := []dns.Zone{
		{
			Name: "schnitzel.org",
			SOA: dns.SOA{
				MName:   "127.0.0.1",
				RName:   "hollondaise.schnitzel.org",
				Serial:  1,
				Refresh: 2,
				Retry:   3,
				Expire:  4,
				TTL:     5,
			},
		},
	}

	ctrl, dnsProvider, cycleCh, updatesCh := getTestDNSController(seed)
	ctrl.Run()
	defer ctrl.Stop()

	eps1, err := TryParseEndpointProtocols("http://127.0.0.1:80,tcp://127.0.1.1:8080")
	if err != nil {
		t.Fatalf("couldn't parse endpoint, see: %v", err)
	}

	newLBs := NewLoadbalancerList([]Loadbalancer{
		NewLoadbalancer("zwiebel.schnitzel.org", eps1...),
	})

	updatesCh <- newLBs

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ := dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err := dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 2 {
		t.Fatalf("expected 2 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "127.0.0.1" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

	if zone.Records[1].Value != "127.0.1.1" ||
		zone.Records[1].Type != dns.RecordTypeA ||
		zone.Records[1].Name != "zwiebel" {
		t.Fatalf("expected second record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[1].Type, zone.Records[1].Name, zone.Records[1].Value)
	}

	waitDNSCycles(cycleCh, 2)

	dnsProvider.UpdateZone(dns.Zone{
		Name: "schnitzel.org",
		SOA: dns.SOA{
			MName:   "127.0.0.1",
			RName:   "hollondaise.schnitzel.org",
			Serial:  1,
			Refresh: 2,
			Retry:   3,
			Expire:  4,
			TTL:     5,
		},
	})

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ = dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err = dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 2 {
		t.Fatalf("expected 2 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "127.0.0.1" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

	if zone.Records[1].Value != "127.0.1.1" ||
		zone.Records[1].Type != dns.RecordTypeA ||
		zone.Records[1].Name != "zwiebel" {
		t.Fatalf("expected second record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[1].Type, zone.Records[1].Name, zone.Records[1].Value)
	}

}

// TestIgnoreForeignZones asserts that we don't fuck up zones we dont have lbs for configured
func TestIgnoreForeignZones(t *testing.T) {
	seed := []dns.Zone{
		{
			Name: "schnitzel.org",
			SOA: dns.SOA{
				MName:   "127.0.0.1",
				RName:   "hollondaise.schnitzel.org",
				Serial:  1,
				Refresh: 2,
				Retry:   3,
				Expire:  4,
				TTL:     5,
			},
		},
		{
			Name: "johnny.org",
			SOA: dns.SOA{
				MName:   "127.0.0.1",
				RName:   "mad.johnny.org",
				Serial:  1,
				Refresh: 2,
				Retry:   3,
				Expire:  4,
				TTL:     5,
			},
			Records: []dns.Record{
				{
					Class: dns.ClassIN,
					Name:  "hollondaise",
					Type:  dns.RecordTypeA,
					Value: "127.0.0.1",
				},
			},
		},
	}

	ctrl, dnsProvider, cycleCh, updatesCh := getTestDNSController(seed)
	ctrl.Run()
	defer ctrl.Stop()

	eps1, err := TryParseEndpointProtocols("http://127.0.0.1:80,tcp://127.0.1.1:8080")
	if err != nil {
		t.Fatalf("couldn't parse endpoint, see: %v", err)
	}

	newLBs := NewLoadbalancerList([]Loadbalancer{
		NewLoadbalancer("zwiebel.schnitzel.org", eps1...),
	})

	updatesCh <- newLBs

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ := dnsProvider.ListZones()
	if len(zoneNames) != 2 {
		t.Fatalf("expected 2 zoneNames but got `%d`", len(zoneNames))
	}

	zone, err := dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 2 {
		t.Fatalf("expected 2 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "127.0.0.1" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

	if zone.Records[1].Value != "127.0.1.1" ||
		zone.Records[1].Type != dns.RecordTypeA ||
		zone.Records[1].Name != "zwiebel" {
		t.Fatalf("expected second record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[1].Type, zone.Records[1].Name, zone.Records[1].Value)
	}

	zone, err = dnsProvider.GetZone("johnny.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if len(zone.Records) != 1 {
		t.Fatalf("expected 1 record but got `%d`", len(zone.Records))
	}

	rec := zone.Records[0]

	if zone.SOA != seed[1].SOA {
		t.Fatalf("expected zone SOA to be unchanged")
	}

	if rec.Name != "hollondaise" ||
		rec.Value != "127.0.0.1" ||
		rec.Type != dns.RecordTypeA {
		t.Fatalf("expected foreign record to be unchanged but got `%v`", rec)
	}
}

// TestIgnoreForeignRecords asserts that we don't fuck up records we dont have lbs for configured
func TestIgnoreForeignRecords(t *testing.T) {
	seed := []dns.Zone{
		{
			Name: "schnitzel.org",
			SOA: dns.SOA{
				MName:   "127.0.0.1",
				RName:   "hollondaise.schnitzel.org",
				Serial:  1,
				Refresh: 2,
				Retry:   3,
				Expire:  4,
				TTL:     5,
			},
			Records: []dns.Record{
				{
					Class: dns.ClassIN,
					Name:  "hawaii",
					Type:  dns.RecordTypeA,
					Value: "1.2.3.4",
				},
			},
		},
	}

	ctrl, dnsProvider, cycleCh, updatesCh := getTestDNSController(seed)
	ctrl.Run()
	defer ctrl.Stop()

	eps1, err := TryParseEndpointProtocols("http://127.0.0.1:80,tcp://127.0.1.1:8080")
	if err != nil {
		t.Fatalf("couldn't parse endpoint, see: %v", err)
	}

	newLBs := NewLoadbalancerList([]Loadbalancer{
		NewLoadbalancer("zwiebel.schnitzel.org", eps1...),
	})

	updatesCh <- newLBs

	waitDNSCycles(cycleCh, 2)

	zoneNames, _ := dnsProvider.ListZones()
	if len(zoneNames) != 1 {
		t.Fatalf("expected 1 zoneName but got `%d`", len(zoneNames))
	}

	zone, err := dnsProvider.GetZone("schnitzel.org")
	if err != nil {
		t.Fatalf("couldn't get zone, see: %v", zone)
	}

	if zone.SOA != seed[0].SOA {
		t.Fatalf("expected SOA to be equal")
	}

	if zone.Name != seed[0].Name {
		t.Fatalf("expected zone name `%s` to be `%s`", zone.Name, seed[0].Name)
	}

	if len(zone.Records) != 3 {
		t.Fatalf("expected 3 records but got `%d`", len(zone.Records))
	}

	if zone.Records[0].Value != "1.2.3.4" ||
		zone.Records[0].Type != dns.RecordTypeA ||
		zone.Records[0].Name != "hawaii" {
		t.Fatalf("expected second record to be a A record with name `hawaii` to `1.2.3.4` but got `%s` record with `%s` name to `%s`", zone.Records[0].Type, zone.Records[0].Name, zone.Records[0].Value)
	}

	if zone.Records[1].Value != "127.0.0.1" ||
		zone.Records[1].Type != dns.RecordTypeA ||
		zone.Records[1].Name != "zwiebel" {
		t.Fatalf("expected first record to be a A record with name `zwiebel` to `127.0.0.1` but got `%s` record with `%s` name to `%s`", zone.Records[1].Type, zone.Records[1].Name, zone.Records[1].Value)
	}

	if zone.Records[2].Value != "127.0.1.1" ||
		zone.Records[2].Type != dns.RecordTypeA ||
		zone.Records[2].Name != "zwiebel" {
		t.Fatalf("expected second record to be a A record with name `zwiebel` to `127.0.1.1` but got `%s` record with `%s` name to `%s`", zone.Records[2].Type, zone.Records[2].Name, zone.Records[2].Value)
	}
}

func getTestDNSController(seed []dns.Zone) (*DNSController, *dnsmock.Provider, chan time.Time, chan *LoadbalancerList) {
	seedCopy := make([]dns.Zone, len(seed))

	for i, z := range seed {
		seedCopy[i] = z.Copy()
	}

	updatesCh := make(chan *LoadbalancerList, 0)
	cycleCh := make(chan time.Time, 0)
	intervals := &DNSControllerIntervalOpts{
		CurrentZoneSyncEnforce: time.Second / 2,
		Sync:                   time.Second / 2,
	}

	dnsProvider := dnsmock.NewProvider(seedCopy)

	return NewDNSController(dnsProvider, updatesCh, nil, cycleCh, intervals), dnsProvider, cycleCh, updatesCh
}

func waitDNSCycles(ch chan time.Time, n int) {
	for i := 0; n < 0 || i < n; i++ {
		<-ch
	}
}
