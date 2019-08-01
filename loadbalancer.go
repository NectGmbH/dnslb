package main

import (
    "encoding/binary"
    "fmt"

    "github.com/OneOfOne/xxhash"
)

// LoadbalancerList is a list of loadbalancers with a checksum over the whole list
type LoadbalancerList struct {
    Loadbalancers []Loadbalancer
    Hash          uint64
}

func (lbs *LoadbalancerList) String() string {
    return fmt.Sprintf("(%d) %v", lbs.Hash, lbs.Loadbalancers)
}

// NewLoadbalancerList creates a new list with all passed loadbalancers and calculates a hash for it.
func NewLoadbalancerList(lbs []Loadbalancer) *LoadbalancerList {
    list := &LoadbalancerList{Loadbalancers: lbs}
    list.Hash = list.calculateHash()

    return list
}

// Loadbalancer represents a dns loadbalancer with healthchecked endpoints
type Loadbalancer struct {
    Name      string
    Endpoints []EndpointProtocol
}

func (lb Loadbalancer) String() string {
    return fmt.Sprintf("%s %v", lb.Name, lb.Endpoints)
}

func (lbs *LoadbalancerList) calculateHash() uint64 {
    h := xxhash.New64()

    for _, lb := range lbs.Loadbalancers {
        h.Write([]byte(lb.Name))

        for _, e := range lb.Endpoints {
            h.Write(e.IP)
            buf := make([]byte, 2)
            binary.LittleEndian.PutUint16(buf, e.Port)
            h.Write(buf)
        }
    }

    return h.Sum64()
}
