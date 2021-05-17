package mockjson

import (
	"io/ioutil"
	"testing"

	"github.com/NectGmbH/dns"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func TestCreate(t *testing.T) {
	var mockZonePath string
	mockZonePath = "/home/oms101/home/nect_devops/dnslb/foo.yaml"
	mockBuf, err := ioutil.ReadFile(mockZonePath)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"path":   mockZonePath,
			"reason": err,
		}).Fatal("couldn't read mock znes")
	}

	var zones []dns.Zone
	err = yaml.Unmarshal(mockBuf, &zones)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"path":   mockZonePath,
			"reason": err,
		}).Fatal("couldn't unmarshal mock znes")
	}

	for _, z := range zones {
		logrus.WithField("zone", z.String()).Debug("Got mock zone seed")
	}
	var myZonePath string
	myZonePath = "./here"
	var dnsProvider dns.Provider
	dnsProvider = NewProvider(zones, myZonePath)
	dnsProvider.DeleteZone("sdasdas")
	dnsProvider.DeleteZone("sdasdas2")
}
