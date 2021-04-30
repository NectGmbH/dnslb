package main

import (
	"strings"

	"github.com/NectGmbH/dns"

	"github.com/sirupsen/logrus"
)

type DebugDNSProvider struct {
	p dns.Provider
}

func NewDebugDNSProvider(wrapped dns.Provider) *DebugDNSProvider {
	return &DebugDNSProvider{p: wrapped}
}

// CreateZone creates the passed zone
func (d *DebugDNSProvider) CreateZone(zone dns.Zone) error {
	logrus.WithFields(logrus.Fields{
		"zone": zone.String(),
	}).Debug("Calling CreateZone")

	err := d.p.CreateZone(zone)

	logrus.WithFields(logrus.Fields{
		"zone": zone,
		"err":  err,
	}).Debug("Called CreateZone")

	return err
}

// GetZone retrieves the zone information for the zone with the passed name.
func (d *DebugDNSProvider) GetZone(name string) (dns.Zone, error) {
	logrus.WithFields(logrus.Fields{
		"zone": name,
	}).Debug("Calling GetZone")

	zone, err := d.p.GetZone(name)

	logrus.WithFields(logrus.Fields{
		"zone": zone.String(),
		"err":  err,
	}).Debug("Called GetZone")

	return zone, err
}

// ListZones lists all zones from the provider.
func (d *DebugDNSProvider) ListZones() ([]string, error) {
	logrus.Debug("Calling ListZones")

	zones, err := d.p.ListZones()

	logrus.WithFields(logrus.Fields{
		"err":   err,
		"zones": strings.Join(zones, ","),
	}).Debug("Called ListZones")

	return zones, err
}

// UpdateZone updates the zone on the provider.
func (d *DebugDNSProvider) UpdateZone(zone dns.Zone) error {
	logrus.WithFields(logrus.Fields{
		"zone": zone.String(),
	}).Debug("Calling UpdateZone")

	err := d.p.UpdateZone(zone)

	logrus.WithFields(logrus.Fields{
		"err": err,
	}).Debug("Called UpdateZone")

	return err
}

// DeleteZone deletes the passed zone from the provider.
func (d *DebugDNSProvider) DeleteZone(name string) error {
	logrus.WithFields(logrus.Fields{
		"zone": name,
	}).Debug("Calling DeleteZone")

	err := d.p.DeleteZone(name)

	logrus.WithFields(logrus.Fields{
		"zone": name,
		"err":  err,
	}).Debug("Called DeleteZone")

	return err
}
