package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/http/httputil"

    "github.com/NectGmbH/dns"
    "github.com/NectGmbH/dns/provider/autodns"

    "github.com/motemen/go-loghttp"
    "github.com/sirupsen/logrus"
    "gopkg.in/yaml.v3"
)

// ProviderNameAutoDNS is the name of the autodns provider
const ProviderNameAutoDNS = "autodns"

// ProviderNames contains a list of all valid provider names.
var ProviderNames = []string{
    ProviderNameAutoDNS,
}

func main() {
    // - Parsing: Action -------------------------------------------------------
    var getAction string
    var listAction bool
    var deleteAction string
    var updateAction string
    flag.StringVar(&getAction, "get", "", "Action, dumps the zone with the specified name. Only one action at a time can be executed.")
    flag.BoolVar(&listAction, "list", false, "Action, dumps all zones. Only one action at a time can be executed")
    flag.StringVar(&deleteAction, "delete", "", "Action, deletes the zones with the specified name. Only one action at a time can be executed.")
    flag.StringVar(&updateAction, "update", "", "Action, updates the zone using the specified file. Only one action at a time can be executed.")

    // - Parsing: General ------------------------------------------------------
    var provider string
    var dumpHTTP bool
    flag.StringVar(&provider, "provider", "", fmt.Sprintf("name of the provider, currently supported: %#v", ProviderNames))
    flag.BoolVar(&dumpHTTP, "dump-http", false, fmt.Sprintf("flag indicating whether all http requests and responses should be dumped"))

    // - Parsing: AutoDNS ------------------------------------------------------
    var autoDNSUsername string
    var autoDNSPassword string
    flag.StringVar(&autoDNSUsername, "autodns-username", "", "username to auth against autodns")
    flag.StringVar(&autoDNSPassword, "autodns-password", "", "password to auth against autodns")

    flag.Parse()

    // - Validation: Action ----------------------------------------------------
    noActions := 0
    if getAction != "" {
        noActions++
    }
    if listAction {
        noActions++
    }
    if deleteAction != "" {
        noActions++
    }
    if updateAction != "" {
        noActions++
    }

    if noActions != 1 {
        logrus.Fatalf("none or multiple actions given, please give exactly one of these args: -get ZONENAME -list -delete ZONENAME -update PATH_TO_ZONE_FILE")
    }

    // - Validation: General ---------------------------------------------------
    if !strInStrSlice(provider, ProviderNames) {
        logrus.Fatalf("unknown provider `%s`, expected one of these: %v", provider, ProviderNames)
    }

    // - Validation: AutoDNS ---------------------------------------------------
    if provider == ProviderNameAutoDNS {
        if autoDNSUsername == "" {
            logrus.Fatalf("missing -autodns-username parameter")
        }

        if autoDNSPassword == "" {
            logrus.Fatalf("missing -autodns-password parameter")
        }
    }

    // - Setup Debugging -------------------------------------------------------
    initDumpHTTP(dumpHTTP)

    // - Setup Provider --------------------------------------------------------
    var dnsProvider dns.Provider
    if provider == ProviderNameAutoDNS {
        dnsProvider = autodns.NewProvider(autoDNSUsername, autoDNSPassword)
    }

    // - Handle Action ---------------------------------------------------------
    if getAction != "" {
        zone, err := dnsProvider.GetZone(getAction)
        if err != nil {
            logrus.Fatalf("couldn't get zone `%s` using provider `%s`, see: %v", getAction, provider, err)
        }

        dump(zone)
    } else if listAction {
        zones, err := dnsProvider.ListZones()
        if err != nil {
            logrus.Fatalf("couldn't list zones using provider `%s`, see: %v", provider, err)
        }

        dump(zones)
    } else if deleteAction != "" {
        err := dnsProvider.DeleteZone(deleteAction)
        if err != nil {
            logrus.Fatalf("couldn't delete zone `%s` using provider `%s`, see: %v", deleteAction, provider, err)
        }

        logrus.Infof("deleted zone `%s`.", deleteAction)
    } else if updateAction != "" {
        buf, err := ioutil.ReadFile(updateAction)
        if err != nil {
            logrus.Fatalf("couldn't update zone using provider `%s`, see: couldn't read file `%s`, see: %v", provider, updateAction, err)
        }

        var zone dns.Zone
        err = yaml.Unmarshal(buf, &zone)
        if err != nil {
            logrus.Fatalf("couldn't update zone using provider `%s`, see: couldn't deserialize file `%s` as yaml, see: %v", provider, updateAction, err)
        }

        err = dnsProvider.UpdateZone(zone)
        if err != nil {
            logrus.Fatalf("couldn't update zone `%s` using provider `%s` from file `%s`, see: %v", zone.Name, provider, updateAction, err)
        }

        logrus.Infof("updated zone `%s`.", zone.Name)
    }
}

func dump(obj interface{}) {
    buf, err := yaml.Marshal(obj)
    if err != nil {
        logrus.Fatalf("couldn't dump object, see: %v", err)
    }

    fmt.Print(string(buf))
}

func initDumpHTTP(dumpHTTP bool) {
    // Dump http traffic if specified (-dump-http)
    loghttp.DefaultTransport = &loghttp.Transport{
        Transport: http.DefaultTransport,
        LogRequest: func(req *http.Request) {
            if dumpHTTP {
                buf, err := httputil.DumpRequest(req, true)
                if err != nil {
                    logrus.StandardLogger().Errorf("Error while dumping http request: %v", err)
                    return
                }

                logrus.StandardLogger().Errorf("REQ: %s", string(buf))
            }
        },
        LogResponse: func(resp *http.Response) {
            if dumpHTTP {
                buf, err := httputil.DumpResponse(resp, true)
                if err != nil {
                    logrus.StandardLogger().Errorf("Error while dumping http response: %v", err)
                    return
                }

                logrus.StandardLogger().Errorf("RESP: %s", string(buf))
            }
        },
    }

    http.DefaultTransport = loghttp.DefaultTransport
}
