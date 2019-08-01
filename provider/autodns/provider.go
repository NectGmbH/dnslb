package autodns

import (
    "fmt"

    "github.com/NectGmbH/autodns/client/zone_tasks"
    "github.com/NectGmbH/autodns/models"
    "github.com/NectGmbH/dns"

    httptransport "github.com/go-openapi/runtime/client"
    "github.com/go-openapi/strfmt"
    "github.com/sirupsen/logrus"
)

// Provider represents a provider which is able to manage zones using the AutoDNS API
type Provider struct {
    username string
    password string
    client   *zone_tasks.Client
}

// NewProvider creates a new instance of the autodns provider.
func NewProvider(username string, password string) *Provider {
    transport := httptransport.New("api.autodns.com", "/v1", []string{"https"})
    transport.DefaultAuthentication = httptransport.BasicAuth(username, password)

    formats := strfmt.Default

    return &Provider{
        username: username,
        password: password,
        client:   zone_tasks.New(transport, formats),
    }
}

// CreateZone creates the passed zone
func (p *Provider) CreateZone(zone dns.Zone) error {
    return fmt.Errorf("NOT IMPLEMENTED")
}

// GetZone retrieves the zone information for the zone with the passed name.
func (p *Provider) GetZone(name string) (dns.Zone, error) {
    mZones, err := p.listMZones()
    if err != nil {
        return dns.Zone{}, fmt.Errorf("couldn't get zone `%s`, see: failed to list zones, see: %v", name, err)
    }

    var foundZone *models.Zone

    for _, z := range mZones {
        if z.Origin != nil && *z.Origin == name {
            foundZone = z
        }
    }

    if foundZone == nil {
        return dns.Zone{}, fmt.Errorf("zone `%s` not found", name)
    }

    mZone, err := p.getMZone(*foundZone.Origin, foundZone.VirtualNameServer)
    if err != nil {
        return dns.Zone{}, fmt.Errorf("couldn't get zone `%s`, see: failed to retrieve zone upstream, see: %v", name, err)
    }

    return p.mapToDNSZone(mZone), nil
}

// ListZones lists all zones from the provider.
func (p *Provider) ListZones() ([]string, error) {
    mZones, err := p.listMZones()
    if err != nil {
        return nil, fmt.Errorf("couldn't retrieve zone information from autodns, see: %v", err)
    }

    zones := make([]string, 0)

    for _, mZone := range mZones {
        if mZone.Origin == nil {
            logrus.Warnf("found zone without origin, since we use it as unique identifier, this shouldn't have happend, see: %#v", mZone)
            continue
        }

        zones = append(zones, *mZone.Origin)
    }

    return zones, nil
}

// UpdateZone updates the zone on the provider.
func (p *Provider) UpdateZone(zone dns.Zone) error {
    primaryNS := p.getPrimaryNSForZone(zone)
    if primaryNS == "" {
        return fmt.Errorf("couldn't retrieve primary nameserver for zone `%s`", zone.Name)
    }

    mZone, err := p.getMZone(zone.Name, primaryNS)
    if err != nil {
        return fmt.Errorf("couldn't retrieve zone information for `%s`, see: %v", zone.Name, err)
    }

    p.mapToMZone(zone, mZone)

    context := "4"

    updateParams := zone_tasks.
        NewZoneUpdateParams().
        WithXDomainrobotContext(&context).
        WithName(zone.Name).
        WithNameserver(zone.SOA.MName).
        WithBody(mZone)

    _, err = p.client.ZoneUpdate(updateParams)
    if err != nil {
        return fmt.Errorf("couldn't update zone `%s`, see: couldn't query zoneupdate, see: %v", zone.Name, err)
    }

    return nil
}

// DeleteZone deletes the passed zone from the provider. UNTESTED.
func (p *Provider) DeleteZone(name string) error {
    mZones, err := p.listMZones()
    if err != nil {
        return fmt.Errorf("couldn't delete zone `%s`, see: failed to list zones, see: %v", name, err)
    }

    var foundZone *models.Zone

    for _, z := range mZones {
        if z.Origin != nil && *z.Origin == name {
            foundZone = z
        }
    }

    if foundZone == nil {
        return fmt.Errorf("zone `%s` not found", name)
    }

    context := "4"

    deleteParams := zone_tasks.
        NewZoneDeleteParams().
        WithXDomainrobotContext(&context).
        WithName(*foundZone.Origin).
        WithNameserver(foundZone.VirtualNameServer)

    _, err = p.client.ZoneDelete(deleteParams)
    if err != nil {
        return fmt.Errorf("couldn't delete zone `%s`, see: couldn't query zoneDelete, see: %v", name, err)
    }

    return nil
}

func (p *Provider) getPrimaryNSForZone(zone dns.Zone) string {
    for _, rec := range zone.Records {
        if rec.Type == dns.RecordTypeNS {
            return rec.Value
        }
    }

    return ""
}

func (p *Provider) getMZone(name string, ns string) (*models.Zone, error) {
    context := "4"

    infoParams := zone_tasks.
        NewZoneInfoParams().
        WithXDomainrobotContext(&context).
        WithName(name).
        WithNameserver(ns)

    zInfo, err := p.client.ZoneInfo(infoParams)
    if err != nil {
        return nil, fmt.Errorf("couldn't get zone `%s`, see: couldn't query zoneinfo, see: %v", name, err)
    }

    if zInfo.Payload == nil || len(zInfo.Payload.Data) != 1 {
        return nil, fmt.Errorf("couldn't get zone `%s`, see: ambiguous payload", name)
    }

    return zInfo.Payload.Data[0], nil
}

func (p *Provider) listMZones() ([]*models.Zone, error) {
    query := &models.Query{
        View: &models.QueryView{
            Limit: 50000,
        },
    }

    context := "4"

    params := zone_tasks.
        NewZoneListParams().
        WithXDomainrobotContext(&context).
        WithBody(query)

    resp, err := p.client.ZoneList(params)
    if err != nil {
        return nil, fmt.Errorf("couldn't list zones, see: %v", err)
    }

    return resp.Payload.Data, nil
}

func (p *Provider) mapToMZone(dZone dns.Zone, mZone *models.Zone) {
    mZone.Soa.Refresh = dZone.SOA.Refresh
    mZone.Soa.Retry = dZone.SOA.Retry
    mZone.Soa.Expire = dZone.SOA.Expire
    mZone.Soa.TTL = dZone.SOA.TTL
    mZone.Soa.Email = &dZone.SOA.RName

    mZone.VirtualNameServer = dZone.SOA.MName

    var mainA dns.Record

    mZone.NameServers = make([]*models.NameServer, 0)
    mZone.ResourceRecords = make([]*models.ResourceRecord, 0)

    for _, r := range dZone.Records {
        (func(rec dns.Record) {
            if rec.Type == dns.RecordTypeA && rec.Name == "" && mainA.Value == "" {
                mainA = rec
            } else if rec.Type == dns.RecordTypeNS {
                mNS := &models.NameServer{Name: &rec.Value}
                mZone.NameServers = append(mZone.NameServers, mNS)
            } else {
                mRecord := &models.ResourceRecord{
                    Name:  &rec.Name,
                    TTL:   rec.TTL,
                    Type:  string(rec.Type),
                    Value: rec.Value,
                    Pref:  int32(rec.Preference),
                }

                mZone.ResourceRecords = append(mZone.ResourceRecords, mRecord)
            }
        })(r)
    }

    mZone.Main = &models.MainIP{
        Address: mainA.Value,
        TTL:     mainA.TTL,
    }

    // Grants can't be null or else autodns returns 500 error...
    mZone.Grants = make([]string, 0)
}

func (p *Provider) mapToDNSZone(mZone *models.Zone) dns.Zone {
    zone := dns.Zone{Name: *mZone.Origin}

    if mZone.Soa != nil {
        zone.SOA.Refresh = mZone.Soa.Refresh
        zone.SOA.Retry = mZone.Soa.Retry
        zone.SOA.Expire = mZone.Soa.Expire
        zone.SOA.TTL = mZone.Soa.TTL
        zone.SOA.MName = mZone.VirtualNameServer

        if mZone.Soa.Email != nil {
            zone.SOA.RName = *mZone.Soa.Email
        }
    }

    for _, mNameServer := range mZone.NameServers {
        if mNameServer.Name != nil {
            zone.Records = append(zone.Records, p.newNSRecord(*mNameServer.Name))
        }
    }

    if mZone.Main != nil {
        rec := dns.NewRecord(dns.ClassIN, "", mZone.Main.TTL, dns.RecordTypeA, mZone.Main.Address, 0)
        zone.Records = append(zone.Records, *rec)
    }

    for _, mRecord := range mZone.ResourceRecords {
        zone.Records = append(zone.Records, p.mapToDNSRecord(mRecord))
    }

    return zone
}

func (p *Provider) newNSRecord(ns string) dns.Record {
    return dns.Record{
        Class: dns.ClassIN,
        Type:  dns.RecordTypeNS,
        Value: ns,
    }
}

func (p *Provider) mapToDNSRecord(mRecord *models.ResourceRecord) dns.Record {
    nameStr := ""
    if mRecord.Name != nil {
        nameStr = *mRecord.Name
    }

    return dns.Record{
        Class:      dns.ClassIN,
        Name:       nameStr,
        Preference: int64(mRecord.Pref),
        TTL:        mRecord.TTL,
        Type:       dns.RecordType(mRecord.Type),
        Value:      mRecord.Value,
    }
}
