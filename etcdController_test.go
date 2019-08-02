package main

import (
    "encoding/json"
    "testing"
    "time"

    "github.com/NectGmbH/health"
)

// TestMonitorLoop tests whether the monitors will be pushed to the etcd on start and after upstream changed
func TestMonitorLoop(t *testing.T) {
    ep1 := "http://127.0.0.1:80"
    ep2 := "tcp://127.0.1.1:8080"
    eps, err := TryParseEndpointProtocols(ep1 + "," + ep2)
    if err != nil {
        t.Fatalf("couldn't parse endpoint protocols, see: %v", err)
    }

    lb1 := NewLoadbalancer("lb1", eps...)

    ctrl, etcd := getTestETCDController(t, nil, nil, lb1)
    mockAgentStatus(etcd, "agent1", time.Now(), newStatus(ep1, true), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent2", time.Now(), newStatus(ep1, true), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent3", time.Now(), newStatus(ep1, true), newStatus(ep2, true))

    val, err := etcd.Get("monitors")
    if err.Error() != "no value for key `monitors` found" {
        t.Fatalf("expected monitors to be empty before controller has been started, but it wasnt, err: %v val: %s", err, val)
    }

    ctrl.Run()
    defer ctrl.Stop()

    // FIXME: create cycle chan for monitor updates
    time.Sleep(1 * time.Second)

    expectedMon := "[\"http://127.0.0.1:80\",\"tcp://127.0.1.1:8080\"]"
    val, err = etcd.Get("monitors")
    if val != expectedMon {
        t.Fatalf("expected monitors to be `%s` on start but got `%s` instead (err: %v)", expectedMon, val, err)
    }

    etcd.Set("monitors", "changed")

    time.Sleep(1 * time.Second)
    val, err = etcd.Get("monitors")
    if val != expectedMon {
        t.Fatalf("expected monitors to be `%s` after change but got `%s` instead (err: %v)", expectedMon, val, err)
    }
}

// TestStatusUpdateOnMajority tests whether etcd reports status updates when majority changes
func TestStatusUpdateOnMajority(t *testing.T) {
    ep1 := "http://127.0.0.1:80"
    ep2 := "tcp://127.0.1.1:8080"
    ep3 := "http://1.2.3.4:80"
    epsLb1, err := TryParseEndpointProtocols(ep1 + "," + ep2)
    if err != nil {
        t.Fatalf("couldn't parse endpoint protocols, see: %v", err)
    }

    epsLb2, err := TryParseEndpointProtocols(ep3)
    if err != nil {
        t.Fatalf("couldn't parse endpoint protocols, see: %v", err)
    }

    lb1 := NewLoadbalancer("lb1", epsLb1...)
    lb2 := NewLoadbalancer("lb2", epsLb2...)

    cycleCh := make(chan struct{})
    var lbList *LoadbalancerList
    updateLBList := func(lbs *LoadbalancerList) {
        lbList = lbs
    }

    ctrl, etcd := getTestETCDController(t, updateLBList, updateCycleFn(cycleCh), lb1, lb2)
    mockAgentStatus(etcd, "agent1", time.Now(), newStatus(ep1, false), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent2", time.Now(), newStatus(ep1, false), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent3", time.Now(), newStatus(ep1, true), newStatus(ep2, false))

    ctrl.Run()
    defer ctrl.Stop()

    waitCycles(cycleCh, 2)

    if len(lbList.Loadbalancers) != 1 {
        t.Fatalf("expected 1 loadbalancer but got %d", len(lbList.Loadbalancers))
    }

    lb := lbList.Loadbalancers[0]

    if lb.Name != lb1.Name {
        t.Fatalf("expected lb to be lb1 but it was `%s`", lb.Name)
    }

    if len(lb.Endpoints) != 1 {
        t.Fatalf("expected 1 endpoint but got %d", len(lb.Endpoints))
    }

    if lb.Endpoints[0].String() != ep2 {
        t.Fatalf("expected alive endpoint to be `%s` but it was `%v`", ep2, lb.Endpoints[0])
    }

    mockAgentStatus(etcd, "agent1", time.Now(), newStatus(ep1, false), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent2", time.Now(), newStatus(ep1, true), newStatus(ep2, true))
    mockAgentStatus(etcd, "agent3", time.Now(), newStatus(ep1, true), newStatus(ep2, false))

    waitCycles(cycleCh, 2)

    lb = lbList.Loadbalancers[0]

    if len(lb.Endpoints) != 2 {
        t.Fatalf("expected 2 endpoints but got %d", len(lb.Endpoints))
    }
}

func TestNoLBWhenUpdateWithoutMajority(t *testing.T) {
    ep1 := "http://127.0.0.1:80"
    ep2 := "tcp://127.0.1.1:8080"
    ep3 := "http://1.2.3.4:80"
    epsLb1, err := TryParseEndpointProtocols(ep1 + "," + ep2)
    if err != nil {
        t.Fatalf("couldn't parse endpoint protocols, see: %v", err)
    }

    epsLb2, err := TryParseEndpointProtocols(ep3)
    if err != nil {
        t.Fatalf("couldn't parse endpoint protocols, see: %v", err)
    }

    lb1 := NewLoadbalancer("lb1", epsLb1...)
    lb2 := NewLoadbalancer("lb2", epsLb2...)

    cycleCh := make(chan struct{})
    var lbList *LoadbalancerList
    updateLBList := func(lbs *LoadbalancerList) {
        lbList = lbs
    }

    ctrl, etcd := getTestETCDController(t, updateLBList, updateCycleFn(cycleCh), lb1, lb2)
    mockAgentStatus(etcd, "agent1", time.Now(), newStatus(ep1, false), newStatus(ep2, true), newStatus(ep3, true))
    mockAgentStatus(etcd, "agent2", time.Now(), newStatus(ep1, false), newStatus(ep2, true), newStatus(ep3, true))
    mockAgentStatus(etcd, "agent3", time.Now(), newStatus(ep1, true), newStatus(ep2, false), newStatus(ep3, true))

    ctrl.Run()
    defer ctrl.Stop()

    waitCycles(cycleCh, 2)

    if len(lbList.Loadbalancers) != 2 {
        t.Fatalf("expected 2 loadbalancer but got %d", len(lbList.Loadbalancers))
    }

    lb := lbList.Loadbalancers[0]

    if lb.Name != lb1.Name {
        t.Fatalf("expected lb to be lb1 but it was `%s`", lb.Name)
    }

    if len(lb.Endpoints) != 1 {
        t.Fatalf("expected 1 endpoint but got %d", len(lb.Endpoints))
    }

    if lb.Endpoints[0].String() != ep2 {
        t.Fatalf("expected alive endpoint to be `%s` but it was `%v`", ep2, lb.Endpoints[0])
    }

    mockAgentStatus(etcd, "agent1", time.Now(), newStatus(ep3, true))
    mockAgentStatus(etcd, "agent2", time.Now().Add(-5*time.Minute), newStatus(ep1, true), newStatus(ep2, false), newStatus(ep3, true))
    mockAgentStatus(etcd, "agent3", time.Now(), newStatus(ep1, true), newStatus(ep2, false), newStatus(ep3, true))

    waitCycles(cycleCh, 2)

    if len(lbList.Loadbalancers) != 1 {
        t.Fatalf("expected 1 loadbalancer but got %d", len(lbList.Loadbalancers))
    }

    lb = lbList.Loadbalancers[0]
    if lb.Name != lb2.Name {
        t.Fatalf("expected lb to be `%s` but got `%s` instead", lb2.Name, lb.Name)
    }
}

func mockAgentStatus(etcd ETCDProvider, agent string, updated time.Time, status ...health.HealthCheckStatus) {
    s := struct {
        Status []health.HealthCheckStatus
        Time   uint64
    }{
        Status: status,
        Time:   uint64(updated.Unix()),
    }

    buf, _ := json.Marshal(s)

    etcd.Set(agent, string(buf))
}

func newStatus(ep string, healthy bool) health.HealthCheckStatus {
    e, _ := TryParseEndpointProtocol(ep)

    return health.HealthCheckStatus{
        IP:      e.IP,
        Port:    int(e.Port),
        Healthy: healthy,
    }
}

func updateCycleFn(cycleCh chan struct{}) func(time.Time) {
    return func(t time.Time) {
        cycleCh <- struct{}{}
    }
}

func waitCycles(ch chan struct{}, n int) {
    for i := 0; n < 0 || i < n; i++ {
        <-ch
    }
}

func getTestETCDController(t *testing.T, onUpdate func(*LoadbalancerList), onCycle func(time.Time), lbs ...Loadbalancer) (*ETCDController, ETCDProvider) {
    agents := []string{"agent1", "agent2", "agent3"}
    etcd := &ETCDMock{}
    etcd.Init(nil)
    updatesChan := make(chan *LoadbalancerList, 0)
    go (func() {
        for u := range updatesChan {
            if onUpdate != nil {
                onUpdate(u)
            }
        }
    })()

    cycleChan := make(chan time.Time, 0)
    go (func() {
        for t := range cycleChan {
            if onCycle != nil {
                onCycle(t)
            }
        }
    })()

    intervals := &ETCDControllerIntervalOpts{
        Status:  time.Second / 2,
        Monitor: time.Second / 2,
    }

    return NewETCDController(agents, etcd, lbs, updatesChan, nil, cycleChan, intervals), etcd
}
