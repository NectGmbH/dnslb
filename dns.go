package dns

import (
    "encoding/binary"
    "fmt"
    "sort"

    "github.com/OneOfOne/xxhash"
)

// Class represents a DNS class. Most likely you want to use ClassIN.
type Class string

const (
    // ClassIN represents the internet dns class
    ClassIN Class = "IN"

    // ClassCS represetns the chaos dns class
    ClassCS Class = "CS"

    // ClassHS represents the hesiod dns class.
    ClassHS Class = "HS"
)

// RecordType represents the differnet dns record types (e.g. A, MX, ...)
type RecordType string

const (
    // RecordTypeA represents host address
    RecordTypeA RecordType = "A"

    // RecordTypeMX represents mail eXchange
    RecordTypeMX RecordType = "MX"

    // RecordTypeCNAME represents canonical name for an alias
    RecordTypeCNAME RecordType = "CNAME"

    // RecordTypeTXT represents descriptive text
    RecordTypeTXT RecordType = "TXT"

    // RecordTypeSRV represents location of service
    RecordTypeSRV RecordType = "SRV"

    // RecordTypePTR represents pointer
    RecordTypePTR RecordType = "PTR"

    // RecordTypeAAAA represents IPv6 host address
    RecordTypeAAAA RecordType = "AAAA"

    // RecordTypeNS represents the nameserver
    RecordTypeNS RecordType = "NS"

    // RecordTypeCAA represents certification authority authorization
    RecordTypeCAA RecordType = "CAA"

    // RecordTypeTLSA represents transport layer security authentication
    RecordTypeTLSA RecordType = "TLSA"

    // RecordTypeNAPTR represents naming authority pointer
    RecordTypeNAPTR RecordType = "NAPTR"

    // RecordTypeSSHFP represents secure shell fingerprint record
    RecordTypeSSHFP RecordType = "SSHFP"

    // RecordTypeLOC represents location information
    RecordTypeLOC RecordType = "LOC"

    // RecordTypeRP represents responsible person
    RecordTypeRP RecordType = "RP"

    // RecordTypeHINFO represents host information
    RecordTypeHINFO RecordType = "HINFO"

    // RecordTypeALIAS represents auto resolved alias
    RecordTypeALIAS RecordType = "ALIAS"

    // RecordTypeDNSKEY represents DNSSEC public key
    RecordTypeDNSKEY RecordType = "DNSKEY"

    // RecordTypeNSEC represents Next Secure
    RecordTypeNSEC RecordType = "NSEC"

    // RecordTypeDS represents delegation signer
    RecordTypeDS RecordType = "DS"
)

// SOA represents the start of Authority record which contains administrative information about the zone
type SOA struct {
    // MName represents the primary master name server for this zone
    MName string

    // RName represents the (encoded) email address of the administrator responsible for this zone
    RName string

    // Serial is the current serialnumber, should change with every change of the zone.
    Serial int64

    // Refresh is the number of seconds after which secondary name servers should query the master for the SOA record, to detect zone changes.
    Refresh int64

    // Retry is the number of seconds after which secondary name servers should retry to request the serial number from the master if the master does not respond.
    Retry int64

    // Expire is the number of seconds after which secondary name servers should stop answering request for this zone if the master does not respond.
    Expire int64

    // TTL is the time to live for purposes of negative caching.
    TTL int64
}

// Zone represents a DNS Zone
type Zone struct {
    Name    string   `yaml:"Name"`
    Records []Record `yaml:"Records"`
    SOA     SOA      `yaml:"SOA"`
}

func (z Zone) String() string {
    recStr := ""

    for i, rec := range z.Records {
        if i > 0 {
            recStr += ", "
        }

        recStr += rec.String()
    }

    return fmt.Sprintf("%s [%s]", z.Name, recStr)
}

// Zones represents a slice of zones
type Zones []*Zone

// Hash creates a hash for the current zones
func (z Zones) Hash() uint64 {
    h := xxhash.New64()

    for _, zone := range z {
        buf := make([]byte, 8)
        binary.LittleEndian.PutUint64(buf, zone.Hash())
        h.Write(buf)
    }

    return h.Sum64()
}

// Copy copies the current zones
func (z Zones) Copy() Zones {
    newZones := make(Zones, len(z))

    for i, zone := range z {
        copy := zone.Copy()
        newZones[i] = &copy
    }

    return newZones
}

// Equals checks if the current zones equals the passed zones.
func (z Zones) Equals(b Zones) bool {
    if len(z) != len(b) {
        return false
    }

    for i, zone := range z {
        if !zone.Equals(b[i]) {
            return false
        }
    }

    return true
}

// Equals checks if the current zone equals the passed zone.
func (z *Zone) Equals(b *Zone) bool {
    if b == nil && z == nil {
        return true
    }

    if b == nil || z == nil {
        return false
    }

    if z.Name != b.Name {
        return false
    }

    if len(z.Records) != len(b.Records) {
        return false
    }

    return z.Hash() == b.Hash()
}

// RemoveRecordsWithName removes all records from the current zone which matches the passed name
func (z *Zone) RemoveRecordsWithName(name string) {
    recs := make([]Record, 0)

    for _, rec := range z.Records {
        if rec.Name != name {
            recs = append(recs, rec)
        }
    }

    z.Records = recs
}

// Hash creates a hash for the current zone
func (z *Zone) Hash() uint64 {
    h := xxhash.New64()

    h.WriteString(z.Name)

    h.WriteString(z.SOA.MName)
    h.WriteString(z.SOA.RName)

    buf := make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(z.SOA.TTL))
    h.Write(buf)

    buf = make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(z.SOA.Expire))
    h.Write(buf)

    buf = make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(z.SOA.Retry))
    h.Write(buf)

    buf = make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(z.SOA.Refresh))
    h.Write(buf)

    buf = make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(z.SOA.Serial))
    h.Write(buf)

    recs := make([]Record, len(z.Records))
    copy(recs, z.Records)

    sort.Slice(recs, func(i, j int) bool {
        return recs[i].String() < recs[j].String()
    })

    for _, rec := range recs {
        h.WriteString(rec.String())
    }

    return h.Sum64()
}

// Copy creates a deep-copy of the current zone structure.
func (z *Zone) Copy() Zone {
    recs := make([]Record, len(z.Records))
    copy(recs, z.Records)

    return Zone{
        Name:    z.Name,
        SOA:     z.SOA,
        Records: recs,
    }
}

// NewZone creates a new Zone using the passed parameters.
func NewZone(name string, records []Record) *Zone {
    return &Zone{
        Name:    name,
        Records: records,
    }
}

// Record represents a dns zone record
type Record struct {
    Class      Class      `yaml:"Class"`
    Name       string     `yaml:"Name"`
    TTL        int64      `yaml:"TTL"`
    Type       RecordType `yaml:"Type"`
    Value      string     `yaml:"Value"`
    Preference int64      `yaml:"Preference"`
}

func (r Record) String() string {
    return fmt.Sprintf("%s %d %s %s %d %s", r.Name, r.TTL, r.Class, r.Type, r.Preference, r.Value)
}

// NewRecord creates a new DNS zone record
func NewRecord(class Class, name string, ttl int64, rType RecordType, value string, preference int64) *Record {
    return &Record{
        Class:      class,
        Name:       name,
        TTL:        ttl,
        Type:       rType,
        Value:      value,
        Preference: preference,
    }
}
