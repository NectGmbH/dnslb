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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NectGmbH/dns"
	"github.com/NectGmbH/dns/provider/autodns"
	mockdns "github.com/NectGmbH/dns/provider/mock"
	mockjson "github.com/NectGmbH/dns/provider/mockjson"

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

// LeaderElectionImplementationBully for internalbully leader election implementations.
const LeaderElectionImplementationBully = "bully"

// LeaderElectionImplementation contains a list of all valid leader election implementations.
var LeaderElectionImplementation = []string{
	LeaderElectionImplementationSingleton,
	LeaderElectionImplementationK8s,
	LeaderElectionImplementationRaft,
	LeaderElectionImplementationBully,
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

func main() {
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
	flag.Var(&bullyPeers, "bully-peer", "Peer as 'identifier:address'")

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

	if jsonLogging == true {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// - Validation: General ---------------------------------------------------
	if !strInStrSlice(provider, ProviderNames) {
		logrus.Fatalf("unknown provider `%s`, expected one of these: %v", provider, ProviderNames)
	}

	if len(etcds) == 0 {
		logrus.Fatal("no etcds given, pass them using -etcd")
	}

	switch election {
	case LeaderElectionImplementationSingleton:
	case LeaderElectionImplementationK8s:
		if instanceID == "" {
			logrus.Fatalf("no instance id specified, pass it using -instance-id")
		}

		if leaseLockName == "" {
			logrus.Fatalf("no lock name specified, pass it using -k8s-lock-name")
		}

		if leaseLockNamespace == "" {
			logrus.Fatalf("no lock namespace specified, pass it using -k8s-lock-namespace")
		}
	case LeaderElectionImplementationRaft:
		if instanceID == "" {
			logrus.Fatalf("no instance id specified, pass it using -instance-id")
		}
		if raftAddress == "" {
			logrus.Fatalf("no raft-address specified, pass it using -raft-address")
		}
		if raftDir == "" {
			logrus.Fatalf("no raft-dir id specified, pass it using -raft-dir")
		}
	case LeaderElectionImplementationBully:
		if bullyAddress == "" {
			logrus.Fatalf("no bully address specified, pass it using -bully-address")
		}
		if raftAddress == "" {
			logrus.Fatalf("no raft-address specified, pass it using -raft-address")
		}
		if raftDir == "" {
			logrus.Fatalf("no raft-dir id specified, pass it using -raft-dir")
		}
	default:
		logrus.Fatalf("unknown election `%s`, expected one of these: %v", election, LeaderElectionImplementation)
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

	// - Validation: MockDNS ---------------------------------------------------
	if provider == ProviderNameMock {
		if mockZonePath == "" {
			logrus.Fatalf("missing -mock-file parameter")
		}
	}
	// - Validation: MockDNS ---------------------------------------------------
	if provider == ProviderNameMockSerialize {
		if mockZonePath == "" {
			logrus.Fatalf("missing -mock-file parameter")
		}
		if mockZoneStatePath == "" {
			logrus.Fatalf("missing -mock-file-state parameter")
		}
	}

	// - Setup Debugging -------------------------------------------------------
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	initDumpHTTP(dumpHTTP)

	// - Setup Provider --------------------------------------------------------
	var dnsProvider dns.Provider
	if provider == ProviderNameAutoDNS {
		dnsProvider = autodns.NewProvider(autoDNSUsername, autoDNSPassword)
	} else if provider == ProviderNameMock {
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

		dnsProvider = mockdns.NewProvider(zones)
	} else if provider == ProviderNameMockSerialize {
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

		dnsProvider = mockjson.NewProvider(zones, mockZoneStatePath)
	}

	if debug {
		dnsProvider = NewDebugDNSProvider(dnsProvider)
	}

	// - Setup Controllers -----------------------------------------------------
	metrics := &Metrics{}
	err := metrics.Init()
	if err != nil {
		logrus.Fatalf("couldn't initialize metrics, see: %v", err)
	}

	loadbalancers := make([]Loadbalancer, 0)
	for key, value := range lbs {
		lb, err := parseLoadbalancer(key, value)
		if err != nil {
			logrus.Fatalf("couldn't parse endpoints `%+v` for dns zone `%s`, see: %v", value, key, err)
		}

		loadbalancers = append(loadbalancers, lb)
	}

	lbUpdates := make(chan *LoadbalancerList, 0)

	etcd := &ETCD{}
	err = etcd.Init(etcds)
	if err != nil {
		logrus.Fatalf("couldn't connect to etcds `%+v`, see: %v", etcds, err)
	}

	etcdCycleCh := make(chan time.Time, 0)
	etcdCtrl := NewETCDController(agents, etcd, loadbalancers, lbUpdates, metrics, etcdCycleCh, nil)

	dnsCycleCh := make(chan time.Time, 0)
	dnsCtrl := NewDNSController(dnsProvider, lbUpdates, metrics, dnsCycleCh, nil)

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
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		logrus.Fatalf("http server stopped, see: %v", err)
	})()

	// - Setup Leaderelection / start ------------------------------------------
	signalCh := make(chan os.Signal, 1)
	switch election {
	case LeaderElectionImplementationK8s:
		config, err := buildKubeconfig(kubeconfig)
		if err != nil {
			logrus.Fatalf("couldn't build kubeconfig, see: %v", err)
		}

		kubeclient := clientset.NewForConfigOrDie(config)

		// we use the Lease lock type since edits to Leases are less common
		// and fewer objects in the cluster watch "all Leases".
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      leaseLockName,
				Namespace: leaseLockNamespace,
			},
			Client: kubeclient.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: instanceID,
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
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: started leading")

					logrus.Infof("starting controllers")
					etcdCtrl.Run()
					dnsCtrl.Run()

				},
				OnStoppedLeading: func() {
					atomic.SwapInt64(&isLeadingA, 0)
					atomic.SwapInt64(&leadingStoppedAtTS, time.Now().Unix())
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: stopped leading")

					logrus.Infof("stopping controllers")
					dnsCtrl.Stop()
					etcdCtrl.Stop()
				},
				OnNewLeader: func(identity string) {
					if identity == instanceID {
						return
					}

					logrus.WithFields(logrus.Fields{"leader": identity}).Info("leaderelection: new leader elected")
				},
			},
		})

		// because the context is closed, the client should report errors
		_, err = kubeclient.CoordinationV1().Leases(leaseLockNamespace).Get(leaseLockName, metav1.GetOptions{})
		if err == nil || !strings.Contains(err.Error(), "the leader is shutting down") {
			logrus.Fatalf("leaderelection: %s: expected to get an error when trying to make a client call: %v", instanceID, err)
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
		raft := NewRaftController(raftAddress, instanceID, raftDir, raftBootstrap)
		go raft.Run()
		go func() {
			for {
				leading := <-raft.LeaderCh()
				if leading {
					atomic.SwapInt64(&isLeadingA, 1)
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: started leading")

					logrus.Infof("starting controllers")
					etcdCtrl.Run()
					dnsCtrl.Run()
				} else {
					atomic.SwapInt64(&isLeadingA, 0)
					atomic.SwapInt64(&leadingStoppedAtTS, time.Now().Unix())
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: stopped leading")

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
	case LeaderElectionImplementationBully:
		logrus.Info("bully setup")
		bc, err := NewBullyController(instanceID, bullyAddress, bullyProto, bullyPeers)
		if err != nil {
			logrus.Fatalf("couldn't start bully controller, see: %v", err)
		}
		go bc.Run()
		go func() {
			for {
				leading := <-bc.LeaderCh()
				if leading {
					atomic.SwapInt64(&isLeadingA, 1)
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: started leading")

					logrus.Infof("starting controllers")
					etcdCtrl.Run()
					dnsCtrl.Run()
				} else {
					atomic.SwapInt64(&isLeadingA, 0)
					atomic.SwapInt64(&leadingStoppedAtTS, time.Now().Unix())
					logrus.WithFields(logrus.Fields{"leader": instanceID}).Info("leaderelection: stopped leading")

					logrus.Infof("stopping controllers")
					dnsCtrl.Stop()
					etcdCtrl.Stop()
				}
			}
		}()
		<-signalCh
		logrus.Info("Received ^C, shutting down...")
		bc.Stop()
		dnsCtrl.Stop()
		etcdCtrl.Stop()
	}
}

func parseLoadbalancer(key string, value string) (Loadbalancer, error) {
	endpoints, err := TryParseEndpointProtocols(value)
	if err != nil {
		return Loadbalancer{}, fmt.Errorf("couldn't parse EndpointProtocols, see: %v", err)
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
