package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NectGmbH/dns"
	"github.com/NectGmbH/dns/provider/autodns"
	mockdns "github.com/NectGmbH/dns/provider/mock"
	mockjson "github.com/NectGmbH/dns/provider/mockjson"
	"github.com/spf13/viper"

	"gopkg.in/yaml.v3"

	"github.com/motemen/go-loghttp"
	"github.com/namsral/flag"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/transport"
)

// ProviderNameAutoDNS is the name of the autodns provider
const ProviderNameAutoDNS = "autodns"

// ProviderNameMock is the name of the mock dns provider
const ProviderNameMock = "mock"

// ProviderNameMockSerialize is the name of the mock dns provider that dumps its state in json
const ProviderNameMockSerialize = "mockjson"

// ProviderNames contains a list of all valid provider names.
var ProviderNames = []string{
	ProviderNameAutoDNS,
	ProviderNameMock,
	ProviderNameMockSerialize,
}

// LeaderElectionImplementationSingleton contains a list of all valid leader eletion implementations.
const LeaderElectionImplementationSingleton = "singleton"

// LeaderElectionImplementationK8s contains a list of all valid leader eletion implementations.
const LeaderElectionImplementationK8s = "k8s"

// LeaderElectionImplementationRaft for internalraft leader election implementations.
const LeaderElectionImplementationRaft = "raft"

// LeaderElectionImplementation contains a list of all valid leader election implementations.
var LeaderElectionImplementation = []string{
	LeaderElectionImplementationSingleton,
	LeaderElectionImplementationK8s,
	LeaderElectionImplementationRaft,
}

type mapFlags map[string]string

func (i *mapFlags) String() string {
	return "my string representation"
}

func (i *mapFlags) Set(value string) error {
	var ss = strings.Split(value, ":")
	if len(ss) != 3 {
		return errors.New("empty name")
	}
	(*i)[ss[0]] = ss[1] + ":" + ss[2]
	return nil
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
func logConf() {
	logrus.Infof("[info] running with configuration found at %v", viper.ConfigFileUsed())
	keys := viper.AllKeys()
	sort.Strings(keys)
	settings := viper.AllSettings()
	for _, k := range keys {
		logrus.WithFields(logrus.Fields{
			"key":   k,
			"value": settings[k],
		}).Info("dnslb setting")
	}
}

func main() {
	parseCli()
	cfgValidation()
	dnsManage()
}

func parseCli() {

	// - Config: ---------------------------------------------------------------
	var configFile string
	// Note: magic is here if you set parameter to 'config' and its not good magic.
	flag.StringVar(&configFile, "cfg", "dnslb.yaml", fmt.Sprintf("name of the config file"))

	// - Parsing: General ------------------------------------------------------
	var port int
	var debug bool
	var dumpHTTP bool
	var agents StringSlice
	var etcds StringSlice
	var provider string
	var election string
	var jsonLogging bool
	lbs := make(StringMap)
	flag.IntVar(&port, "port", 8080, "port for /metrics and /healthz http endpoints")
	flag.Var(&agents, "agent", "Name of all agents for which we should monitor their status reports. Multiple can be given, e.g.: -agent foo -agent bar")
	flag.Var(&lbs, "lb", "Loadbalancers to use in the format dnsrecord=ip1,ip2. Multiple can be given, e.g.: -lb test.nect.com=http://50.0.0.1:80,tcp://75.0.0.1:443")
	flag.Var(&etcds, "etcd", "etcd endpoint where status should be persisted. Multiple can be given, e.g.: -etcd localhost:2379 -etcd localhost:22379")
	flag.StringVar(&provider, "provider", "", fmt.Sprintf("name of the provider, currently supported: %+v", ProviderNames))
	flag.StringVar(&election, "election", "", fmt.Sprintf("name of the election implementations, currently supported: %+v", LeaderElectionImplementation))
	flag.BoolVar(&dumpHTTP, "dump-http", false, "flag indicating whether all http requests and responses should be dumped")
	flag.BoolVar(&debug, "debug", false, "flag indicating whether debug output should be written")
	flag.BoolVar(&jsonLogging, "json-logging", false, "Always use JSON logging")

	// - Parsing: etcdController-------------------------------------------------
	var etcdMonitorInterval int
	flag.IntVar(&etcdMonitorInterval, "etcd-interval-monitoring", 30, "Seconds between syncing from etcd. Defaults to 30 seconds.")

	// - Parsing: DNSController-------------------------------------------------
	var syncDNSControllerDownload int
	var syncDNSControllerUpload int
	flag.IntVar(&syncDNSControllerDownload, "zone-sync-interval-download", 3600, "DNSController seconds between retrieving current zones from dns provider. Defaults to 1 hour.")
	flag.IntVar(&syncDNSControllerUpload, "zone-sync-interval-upload", 60, "DNSController seconds between uploading to dns provider on change. Defaults to 1 minute.")

	// - Parsing: Kubernetes ---------------------------------------------------
	var instanceID string
	var kubeconfig string
	var leaseLockName string
	var leaseLockNamespace string
	hostname, _ := os.Hostname()

	flag.StringVar(&instanceID, "instance-id", hostname, "instance id for leaderelection. should be unique per instance.")
	flag.StringVar(&kubeconfig, "k8s-kubeconfig", "", "absolut path to the kubeconfig file. Only needed when run outside the cluster. Only needed when -k8s is given")
	flag.StringVar(&leaseLockName, "k8s-lock-name", "dnslb", "the lease lock resource name. Only needed when -k8s is given")
	flag.StringVar(&leaseLockNamespace, "k8s-lock-namespace", "", "the lease lock resource namespace. Only needed when -k8s is given")

	// - Parsing: Raft ---------------------------------------------------
	var raftAddress string
	var raftDir string
	var raftBootstrap bool
	flag.StringVar(&raftAddress, "raft-address", "", "address to listen for raft requests.")
	flag.StringVar(&raftDir, "raft-dir", "", "directory to store raft state. Must have subdirectory of the instance id in it.")
	flag.BoolVar(&raftBootstrap, "raft-bootstrap", false, "bootstrap the raft cluster")

	// - Parsing: Bully ---------------------------------------------------
	var bullyAddress string
	var bullyProto string
	var bullyPeers = mapFlags(make(map[string]string))
	flag.StringVar(&bullyAddress, "bully-address", "", "Address for bully connections")
	flag.StringVar(&bullyProto, "bully-proto", "tcp4", "Protocol for bully connections")
	flag.Var(&bullyPeers, "bully-peer", "Peer as 'identifier:address'. Can be specified more than once.")

	// - Parsing: AutoDNS ------------------------------------------------------
	var autoDNSUsername string
	var autoDNSPassword string
	flag.StringVar(&autoDNSUsername, "autodns-username", "", "username to auth against autodns")
	flag.StringVar(&autoDNSPassword, "autodns-password", "", "password to auth against autodns")

	// - Parsing: Mocked DNS ---------------------------------------------------
	var mockZonePath string
	flag.StringVar(&mockZonePath, "mock-file", "", "file containing yaml encoded []dns.Zone")
	// - Parsing: Mocked DNS ---------------------------------------------------
	var mockZoneStatePath string
	flag.StringVar(&mockZoneStatePath, "mock-file-state", "", "json encoded []dns.Zone and updates counter")

	flag.Parse()

	// - set defaults in viper
	viper.SetDefault("port", 8080)
	viper.SetDefault("dump-http", false)
	viper.SetDefault("debug", false)
	viper.SetDefault("json-logging", false)
	viper.SetDefault("zone-sync-interval-download", 3600)
	viper.SetDefault("zone-sync-interval-upload", 60)
	viper.SetDefault("instance-identifier", hostname)
	viper.SetDefault("k8s-lock-name", "dnslb")
	// - Parsing: General ------------------------------------------------------

	if isFlagPassed("cfg") {
		viper.SetConfigName(configFile)       // name of config file (without extension)
		viper.SetConfigType("yaml")           // REQUIRED if the config file does not have the extension in the name
		viper.AddConfigPath("/etc/appname/")  // path to look for the config file in
		viper.AddConfigPath("$HOME/.appname") // call multiple times to add many search paths
		viper.AddConfigPath(".")              // optionally look for config in the working directory
		errViper := viper.ReadInConfig()      // Find and read the config file
		if errViper != nil {                  // Handle errors reading the config file
			logrus.Warn(fmt.Errorf("warnign  config file: %s", errViper))
		}
	}

	if isFlagPassed("json-logging") {
		viper.Set("json-logging", jsonLogging)
	}

	if viper.Get("json-logging") == true {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	if isFlagPassed("port") {
		viper.Set("port", int(port))
	}
	if isFlagPassed("agent") {
		viper.Set("agents", []string(agents))
	}
	if isFlagPassed("lb") {
		vlbs := make(map[string][]string)
		for key, value := range lbs {
			splitted := strings.Split(value, ",")
			vlbs[key] = splitted
		}
		viper.Set("lb", vlbs)
	}
	if isFlagPassed("provider") {
		viper.Set("dnsProvider", provider)
	}
	if isFlagPassed("etcd") {
		viper.Set("etcEndpoint", []string(etcds))
	}
	if isFlagPassed("etcd-interval-monitoring") {
		viper.Set("etcd-interval-monitoring", int(etcdMonitorInterval))
	}
	if isFlagPassed("election") {
		viper.Set("election", string(election))
	}
	if isFlagPassed("dump-http") {
		viper.Set("dump-http", bool(dumpHTTP))
	}
	if isFlagPassed("debug") {
		viper.Set("debug", bool(debug))
	}

	if isFlagPassed("zone-sync-interval-download") {
		viper.Set("zone-sync-interval-download", int(syncDNSControllerDownload))
	}
	if isFlagPassed("zone-sync-interval-upload") {
		viper.Set("zone-sync-interval-upload", int(syncDNSControllerUpload))
	}
	if isFlagPassed("mock-file") {
		viper.Set("mockZonePath", string(mockZonePath))
	}
	if isFlagPassed("mock-file-state") {
		viper.Set("mockZoneStatePath", string(mockZoneStatePath))
	}
	if isFlagPassed("instance-id") {
		viper.Set("instance-identifier", string(instanceID))
	}
	if isFlagPassed("k8s-kubeconfig") {
		viper.Set("k8s-kubeconfig", string(kubeconfig))
	}
	if isFlagPassed("k8s-lock-name") {
		viper.Set("k8s-lock-name", string(leaseLockName))
	}
	if isFlagPassed("k8s-lock-namespace") {
		viper.Set("k8s-lock-namespace", string(leaseLockNamespace))
	}
	if isFlagPassed("raft-address") {
		viper.Set("raft-address", string(raftAddress))
	}
	if isFlagPassed("raft-dir") {
		viper.Set("raft-dir", string(raftDir))
	}
	if isFlagPassed("raft-bootstrap") {
		viper.Set("raft-bootstrap", bool(raftBootstrap))
	}
	if isFlagPassed("autodns-username") {
		viper.Set("autodns-username", string(autoDNSUsername))
	}
	if isFlagPassed("autodns-password") {
		viper.Set("autodns-password", string(autoDNSPassword))
	}

	logConf()

}
func cfgValidation() {
	// - Validation: General ---------------------------------------------------
	if !strInStrSlice(viper.GetString("dnsProvider"), ProviderNames) {
		logrus.Fatalf("unknown provider `%s`, expected one of these: %v", viper.GetString("dnsProvider"), ProviderNames)
	}

	if len(viper.GetStringSlice("etcEndpoint")) == 0 {
		logrus.Fatal("no etcds given, pass them using -etcd")
	}

	switch viper.GetString("election") {
	case LeaderElectionImplementationSingleton:
	case LeaderElectionImplementationK8s:
		if viper.GetString("instance-identifier") == "" {
			logrus.Fatalf("no instance id specified, pass it using -instance-id")
		}

		if viper.GetString("k8s-lock-name") == "" {
			logrus.Fatalf("no lock name specified, pass it using -k8s-lock-name")
		}

		if viper.GetString("k8s-lock-namespace") == "" {
			logrus.Fatalf("no lock namespace specified, pass it using -k8s-lock-namespace")
		}
	case LeaderElectionImplementationRaft:
		if viper.GetString("instance-identifier") == "" {
			logrus.Fatalf("no instance id specified, pass it using -instance-id")
		}
		if viper.GetString("raft-address") == "" {
			logrus.Fatalf("no raft-address specified, pass it using -raft-address")
		}
		if viper.GetString("raft-dir") == "" {
			logrus.Fatalf("no raft-dir id specified, pass it using -raft-dir")
		}
	default:
		logrus.Fatalf("unknown election `%s`, expected one of these: %v", viper.GetString("election"), LeaderElectionImplementation)
	}

	// - Validation: AutoDNS ---------------------------------------------------
	switch viper.GetString("dnsProvider") {
	case ProviderNameAutoDNS:
		if viper.GetString("autodns-username") == "" {
			logrus.Fatalf("missing -autodns-username parameter")
		}

		if viper.GetString("autodns-password") == "" {
			logrus.Fatalf("missing -autodns-password parameter")
		}

	// - Validation: MockDNS ---------------------------------------------------
	case ProviderNameMock:
		if viper.Get("mockZonePath") == "" {
			logrus.Fatalf("missing -mock-file parameter")
		}

	// - Validation: MockDNS ---------------------------------------------------
	case ProviderNameMockSerialize:
		if viper.Get("mockZonePath") == "" {
			logrus.Fatalf("missing -mock-file parameter")
		}
		if viper.Get("mockZoneStatePath") == "" {
			logrus.Fatalf("missing -mock-file-state parameter")
		}
	}

	// - Setup Debugging -------------------------------------------------------
	if viper.GetBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func dnsManage() {
	initDumpHTTP(viper.GetBool("dump-http"))
	// - Setup Provider --------------------------------------------------------
	var dnsProvider dns.Provider
	switch viper.GetString("dnsProvider") {
	case ProviderNameAutoDNS:
		dnsProvider = autodns.NewProvider(viper.GetString("autodns-username"), viper.GetString("autodns-password"))
	case ProviderNameMock:
		mockBuf, err := ioutil.ReadFile(viper.GetString("mockZonePath"))
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   viper.GetString("mockZonePath"),
				"reason": err,
			}).Fatal("couldn't read mock znes")
		}

		var zones []dns.Zone
		err = yaml.Unmarshal(mockBuf, &zones)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   viper.GetString("mockZonePath"),
				"reason": err,
			}).Fatal("couldn't unmarshal mock znes")
		}

		for _, z := range zones {
			logrus.WithField("zone", z.String()).Debug("Got mock zone seed")
		}

		dnsProvider = mockdns.NewProvider(zones)
	case ProviderNameMockSerialize:

		mockBuf, err := ioutil.ReadFile(viper.GetString("mockZonePath"))
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   viper.GetString("mockZonePath"),
				"reason": err,
			}).Fatal("couldn't read mock znes")
		}

		var zones []dns.Zone
		err = yaml.Unmarshal(mockBuf, &zones)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   viper.GetString("mockZonePath"),
				"reason": err,
			}).Fatal("couldn't unmarshal mock znes")
		}

		for _, z := range zones {
			logrus.WithField("zone", z.String()).Debug("Got mock zone seed")
		}

		dnsProvider = mockjson.NewProvider(zones, viper.GetString("mockZoneStatePath"))
	}

	if viper.GetBool("debug") {
		dnsProvider = NewDebugDNSProvider(dnsProvider)
	}

	// - Setup Controllers -----------------------------------------------------
	metrics := &Metrics{}
	metricsErr := metrics.Init()
	if metricsErr != nil {
		logrus.Fatalf("couldn't initialize metrics, see: %v", metricsErr)
	}

	loadbalancers := make([]Loadbalancer, 0)
	viperLb := viper.GetStringMapStringSlice("lb")
	for key, value := range viperLb {
		lb, err := parseLoadbalancerEndpointList(key, value)
		if err != nil {
			logrus.Fatalf("couldn't parse endpoints `%+v` for dns zone `%s`, see: %v", value, key, err)
		}
		loadbalancers = append(loadbalancers, lb)
	}

	lbUpdates := make(chan *LoadbalancerList, 0)

	etcd := &ETCD{}
	err := etcd.Init(viper.GetStringSlice("etcEndpoint"))
	if err != nil {
		logrus.Fatalf("couldn't connect to etcds `%+v`, see: %v", viper.GetStringSlice("etcEndpoint"), err)
	}

	etcdCycleCh := make(chan time.Time, 0)
	etcdIntervalMonitoring := time.Duration(viper.GetInt("etcd-interval-monitoring")) * time.Second
	etcdCtrl := NewETCDController(
		viper.GetStringSlice("agents"),
		etcd,
		loadbalancers,
		lbUpdates,
		metrics,
		etcdCycleCh,
		etcdIntervalMonitoring,
		2*time.Second)

	dnsCycleCh := make(chan time.Time, 0)
	dnsControllerSyncUpload := time.Duration(viper.GetInt("zone-sync-interval-upload")) * time.Second
	dnsControllerSyncDownload := time.Duration(viper.GetInt("zone-sync-interval-download")) * time.Second

	dnsCtrl := NewDNSController(dnsProvider, lbUpdates, metrics, dnsCycleCh, dnsControllerSyncUpload, dnsControllerSyncDownload)

	// - Setup health checking -------------------------------------------------
	var isLeadingA int64
	var leadingStoppedAtTS int64
	var lastDNSCycleTS int64
	var lastETCDCycleTS int64

	go (func() {
		for {
			select {
			case etcdCycle := <-etcdCycleCh:
				atomic.SwapInt64(&lastETCDCycleTS, etcdCycle.Unix())

			case dnsCycle := <-dnsCycleCh:
				atomic.SwapInt64(&lastDNSCycleTS, dnsCycle.Unix())
			}
		}
	})()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		healthy := false
		gracePeriod := 1 * time.Minute

		isLeading := atomic.LoadInt64(&isLeadingA) == 1
		leadingStoppedAt := time.Unix(atomic.LoadInt64(&leadingStoppedAtTS), 0)
		lastDNSCycle := time.Unix(atomic.LoadInt64(&lastDNSCycleTS), 0)
		lastETCDCycle := time.Unix(atomic.LoadInt64(&lastETCDCycleTS), 0)

		// so, we basically want to make sure that healthy means:
		// we are leading -> so our controller should be syncing
		// we aint leading -> so our controller shouldnt be syncing anymore
		if isLeading {
			healthy = time.Since(lastDNSCycle) < gracePeriod && time.Since(lastETCDCycle) < gracePeriod
		} else {
			if time.Since(leadingStoppedAt) > (gracePeriod * 2) {
				healthy = time.Since(lastDNSCycle) > gracePeriod && time.Since(lastETCDCycle) > gracePeriod
			}
		}

		if healthy {
			w.WriteHeader(200)
			w.Write([]byte("{ \"status\": \"healthy\" }"))
		} else {
			w.WriteHeader(500)
			w.Write([]byte("{ \"status\": \"unhealthy\" }"))
		}
	})

	go (func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("port")), nil)
		logrus.Fatalf("http server stopped, see: %v", err)
	})()

	// - Setup Leaderelection / start ------------------------------------------
	signalCh := make(chan os.Signal, 1)
	switch viper.GetString("election") {
	case LeaderElectionImplementationK8s:
		config, err := buildKubeconfig(viper.GetString("k8s-kubeconfig"))
		if err != nil {
			logrus.Fatalf("couldn't build kubeconfig, see: %v", err)
		}

		kubeclient := clientset.NewForConfigOrDie(config)

		// we use the Lease lock type since edits to Leases are less common
		// and fewer objects in the cluster watch "all Leases".
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      viper.GetString("k8s-lock-name"),
				Namespace: viper.GetString("k8s-lock-namespace"),
			},
			Client: kubeclient.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: viper.GetString("instance-identifier"),
			},
		}

		// use a Go context so we can tell the leaderelection code when we
		// want to step down
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// use a client that will stop allowing new requests once the context ends
		config.Wrap(transport.ContextCanceller(ctx, fmt.Errorf("the leader is shutting down")))

		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-signalCh
			logrus.Info("Received termination, signaling shutdown")
			dnsCtrl.Stop()
			etcdCtrl.Stop()
			cancel()
		}()

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   30 * time.Second,
			RenewDeadline:   15 * time.Second,
			RetryPeriod:     5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					atomic.SwapInt64(&isLeadingA, 1)
					logrus.WithFields(logrus.Fields{"leader": viper.GetString("instance-identifier")}).Info("leaderelection: started leading")

					logrus.Infof("starting controllers")
					etcdCtrl.Run()
					dnsCtrl.Run()

				},
				OnStoppedLeading: func() {
					atomic.SwapInt64(&isLeadingA, 0)
					atomic.SwapInt64(&leadingStoppedAtTS, time.Now().Unix())
					logrus.WithFields(logrus.Fields{"leader": viper.GetString("instance-identifier")}).Info("leaderelection: stopped leading")

					logrus.Infof("stopping controllers")
					dnsCtrl.Stop()
					etcdCtrl.Stop()
				},
				OnNewLeader: func(identity string) {
					if identity == viper.GetString("instance-identifier") {
						return
					}

					logrus.WithFields(logrus.Fields{"leader": identity}).Info("leaderelection: new leader elected")
				},
			},
		})

		// because the context is closed, the client should report errors
		_, err = kubeclient.CoordinationV1().Leases(viper.GetString("k8s-lock-namespace")).Get(viper.GetString("k8s-lock-name"), metav1.GetOptions{})
		if err == nil || !strings.Contains(err.Error(), "the leader is shutting down") {
			logrus.Fatalf("leaderelection: %s: expected to get an error when trying to make a client call: %v", viper.GetString("instance-identifier"), err)
		}
	case LeaderElectionImplementationSingleton:
		etcdCtrl.Run()
		dnsCtrl.Run()

		logrus.Info("controller started")

		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		<-signalCh
		logrus.Info("Received ^C, shutting down...")
		dnsCtrl.Stop()
		etcdCtrl.Stop()

		logrus.Info("controller stopped")
	case LeaderElectionImplementationRaft:
		logrus.Info("raft setup")
		raft := NewRaftController(viper.GetString("raft-address"), viper.GetString("instance-identifier"), viper.GetString("raft-dir"), viper.GetBool("raft-bootstrap"))
		go raft.Run()
		go func() {
			for {
				leading := <-raft.LeaderCh()
				if leading {
					atomic.SwapInt64(&isLeadingA, 1)
					logrus.WithFields(logrus.Fields{"leader": viper.GetString("instance-identifier")}).Info("leaderelection: started leading")

					logrus.Infof("starting controllers")
					etcdCtrl.Run()
					dnsCtrl.Run()
				} else {
					atomic.SwapInt64(&isLeadingA, 0)
					atomic.SwapInt64(&leadingStoppedAtTS, time.Now().Unix())
					logrus.WithFields(logrus.Fields{"leader": viper.GetString("instance-identifier")}).Info("leaderelection: stopped leading")

					logrus.Infof("stopping controllers")
					dnsCtrl.Stop()
					etcdCtrl.Stop()
				}
			}
		}()
		<-signalCh
		logrus.Info("Received ^C, shutting down...")
		raft.Stop()
		dnsCtrl.Stop()
		etcdCtrl.Stop()
	}
}

func parseLoadbalancerEndpointList(key string, values []string) (Loadbalancer, error) {
	endpoints := make([]EndpointProtocol, 0)
	for _, s := range values {
		ep, err := TryParseEndpointProtocol(s)
		if err != nil {
			return Loadbalancer{}, fmt.Errorf("couldn't parse `%s` as EndpointProtocol, see: %v", s, err)
		}
		endpoints = append(endpoints, ep)
	}
	return Loadbalancer{
		Name:      key,
		Endpoints: endpoints,
	}, nil
}

func buildKubeconfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
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
