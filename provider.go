package dns

// Provider represents an abstraction over individual dns providers handling zone management
type Provider interface {
    // CreateZone creates the passed zone
    CreateZone(zone Zone) error

    // GetZone retrieves the zone information for the zone with the passed name.
    GetZone(name string) (Zone, error)

    // ListZones lists all zones from the provider.
    ListZones() ([]string, error)

    // UpdateZone updates the zone on the provider.
    UpdateZone(zone Zone) error

    // DeleteZone deletes the passed zone from the provider.
    DeleteZone(name string) error
}
