package main

import (
    "encoding/json"
    "fmt"
    "io"
    "strings"
    "time"

    "github.com/NectGmbH/health"
    "github.com/OneOfOne/xxhash"
    "github.com/sirupsen/logrus"
)

// ETCDControllerIntervalOpts define overrides for the intervals
type ETCDControllerIntervalOpts struct {
    Status  time.Duration
    Monitor time.Duration
}

// ETCDController represents a controller which checks the upstream etcd health statuses and updated the local state
type ETCDController struct {
    metrics            *Metrics
    agents             []string
    wanted             []Loadbalancer
    current            []Loadbalancer
    lastMonitorHash    uint64
    updates            chan *LoadbalancerList
    monitorsStopCh     chan struct{}
    statusStopCh       chan struct{}
    monitorLoopStarted bool
    statusLoopStarted  bool
    statusInterval     time.Duration
    monitorInterval    time.Duration
    maxStatusAge       time.Duration
    running            bool
    lastCycle          chan time.Time
    etcd               ETCDProvider
}

// NewETCDController creates a new controller
func NewETCDController(agents []string, etcd ETCDProvider, wantedLBs []Loadbalancer, updates chan *LoadbalancerList, metrics *Metrics, lastCycle chan time.Time, intervals *ETCDControllerIntervalOpts) *ETCDController {
    ctrl := &ETCDController{
        wanted:          wantedLBs,
        updates:         updates,
        agents:          agents,
        statusInterval:  2 * time.Second,
        monitorInterval: 30 * time.Second,
        maxStatusAge:    45 * time.Second,
        metrics:         metrics,
        lastCycle:       lastCycle,
        etcd:            etcd,
    }

    if intervals != nil {
        ctrl.statusInterval = intervals.Status
        ctrl.monitorInterval = intervals.Monitor
    }

    return ctrl
}

// Run starts the controller in a async, non blocking way
func (c *ETCDController) Run() {
    if c.running {
        return
    }

    c.running = true

    c.monitorsStopCh = make(chan struct{}, 0)
    c.statusStopCh = make(chan struct{}, 0)

    go c.monitorLoop()
    go c.statusLoop()
}

func (c *ETCDController) statusLoop() {
    c.statusLoopStarted = true
    defer (func() { c.statusLoopStarted = false })()
    logrus.Infof("status loop started.")

    for {
        select {
        case <-c.statusStopCh:
            close(c.statusStopCh)
            logrus.Infof("status loop stopped.")
            return
        default:
            now := time.Now()
            logrus.Debugf("etcd-controller: started syncing status")
            err := c.syncStatus()
            if err != nil {
                if c.metrics != nil {
                    c.metrics.ErrorsTotal.Inc()
                    c.metrics.ErrorsETCD.Inc()
                }

                logrus.Errorf("etcd-controller: failed syncing status, see: %v", err)

            } else {
                duration := time.Since(now)
                logrus.Debugf("etcd-controller: finished syncing status in %v", duration)

                if c.metrics != nil {
                    c.metrics.ETCDSyncTime.Observe(duration.Seconds() * 1e3)
                }
            }

            c.lastCycle <- time.Now()

            time.Sleep(c.statusInterval)
        }
    }
}

func (c *ETCDController) syncStatus() error {
    // - Retrieve status for all known agents ----------------------------------
    endpointToHealthy := make(map[string]int)

    if c.metrics != nil {
        c.metrics.AgentsTotal.Set(float64(len(c.agents)))
    }

    downAgents := 0

    for _, agent := range c.agents {
        statuses, err := c.getStatusFromAgent(agent)
        if err != nil {

            if c.metrics != nil {
                c.metrics.AgentsToHealthyEndpoints.WithLabelValues(agent).Set(0)
            }

            downAgents++
            logrus.Errorf("couldn't get status for agent `%s`, see: %v", agent, err)
            continue
        }

        numHealthy := float64(0)

        for _, status := range statuses {
            key := fmt.Sprintf("%v:%d", status.IP, status.Port)

            if !status.Healthy {
                continue
            }

            numHealthy++
            endpointToHealthy[key] = endpointToHealthy[key] + 1
        }

        if c.metrics != nil {
            c.metrics.AgentsToHealthyEndpoints.WithLabelValues(agent).Set(numHealthy)
        }
    }

    upAgents := len(c.agents) - downAgents

    if c.metrics != nil {
        c.metrics.AgentsUp.Set(float64(upAgents))
        c.metrics.AgentsDown.Set(float64(downAgents))
    }

    // - Map to LBs & take only endpoints/lbs where majority think its healthy -
    current := make([]Loadbalancer, 0)
    for _, wantedLB := range c.wanted {
        currentLB := Loadbalancer{Name: wantedLB.Name}
        healthyEP := float64(0)
        unhealthyEP := float64(0)

        for _, endpoint := range wantedLB.Endpoints {
            key := endpoint.Endpoint.String()
            amountHealthy := endpointToHealthy[key]
            majorityHealthy := amountHealthy > (len(c.agents) / 2)

            if !majorityHealthy {
                logrus.WithFields(logrus.Fields{
                    "loadbalancer":      currentLB.Name,
                    "endpoint":          endpoint.String(),
                    "amountHealthy":     amountHealthy,
                    "amountUnk":         len(c.agents) - amountHealthy,
                    "neededForMajority": (len(c.agents) / 2) + 1,
                }).Warnf("majority thinks endpoint is down")

                unhealthyEP++
                continue
            }

            healthyEP++

            currentLB.Endpoints = append(currentLB.Endpoints, endpoint)
        }

        if c.metrics != nil {
            c.metrics.LBTotalEndpoints.WithLabelValues(currentLB.Name).Set(float64(len(wantedLB.Endpoints)))
            c.metrics.LBHealthyEndpoints.WithLabelValues(currentLB.Name).Set(healthyEP)
            c.metrics.LBUnhealthyEndpoints.WithLabelValues(currentLB.Name).Set(unhealthyEP)
        }

        if len(currentLB.Endpoints) == 0 {
            logrus.WithFields(logrus.Fields{
                "loadbalancer": currentLB.Name,
            }).Warnf("majority can't reach a single endpoint of lb, will ignore it.")

            continue
        }

        current = append(current, currentLB)
    }

    logrus.Debugf("syncStatus: current: %+v wanted: %+v", current, c.wanted)

    // - Pass lbs to dns controller using updates chan -------------------------
    c.current = current
    go (func(lbs []Loadbalancer) { c.updates <- NewLoadbalancerList(lbs) })(current)

    return nil
}

func (c *ETCDController) getStatusFromAgent(agent string) ([]health.HealthCheckStatus, error) {
    val, err := c.etcd.Get(agent)
    if err != nil {
        return nil, fmt.Errorf("couldn't retrieve status for agent `%s` from etcd, see: %v", agent, err)
    }

    var status struct {
        Status []health.HealthCheckStatus
        Time   uint64
    }

    err = json.Unmarshal([]byte(val), &status)
    if err != nil {
        return nil, fmt.Errorf("couldn't deserialize status for agent `%s`, see: %v", agent, err)
    }

    t := time.Unix(int64(status.Time), 0)
    since := time.Since(t)

    if c.metrics != nil {
        c.metrics.AgentsStatusAge.WithLabelValues(agent).Set(since.Seconds())
    }

    if since > c.maxStatusAge {
        return nil, fmt.Errorf(
            "most recent status from agent `%s` is `%s` old which exceeds the maximum of `%s` so we don't trust this agent",
            agent,
            since.String(),
            c.maxStatusAge.String())
    }

    return status.Status, nil
}

func (c *ETCDController) monitorLoop() {
    c.monitorLoopStarted = true
    defer (func() { c.monitorLoopStarted = false })()
    logrus.Infof("monitor loop started.")

    for {
        select {
        case <-c.monitorsStopCh:
            close(c.monitorsStopCh)
            logrus.Infof("monitor loop stopped.")
            return
        default:
            now := time.Now()
            logrus.Debugf("etcd-controller: started syncing monitors")
            err := c.syncMonitors()
            if err != nil {
                if c.metrics != nil {
                    c.metrics.ErrorsTotal.Inc()
                    c.metrics.ErrorsETCD.Inc()
                }

                logrus.Errorf("etcd-controller: failed syncing monitors, see: %v", err)

            } else {
                duration := time.Since(now)
                logrus.Debugf("etcd-controller: finished syncing monitors in %v", duration)

                if c.metrics != nil {
                    c.metrics.ETCDMonitorSyncTime.Observe(duration.Seconds() * 1e3)
                }
            }

            time.Sleep(c.monitorInterval)
        }
    }
}

func (c *ETCDController) getWantedMonitorsAsString() (string, error) {
    monitors := make([]string, 0)

    for _, lb := range c.wanted {
        for _, out := range lb.Endpoints {
            monitor := fmt.Sprintf("%s://%v:%d", out.Protocol, out.Endpoint.IP, out.Endpoint.Port)
            monitors = append(monitors, monitor)
        }
    }

    buf, err := json.Marshal(monitors)
    if err != nil {
        return "", fmt.Errorf("couldn't serialize monitors `%+v` to json, see: %v", monitors, err)
    }

    return string(buf), nil
}

func (c *ETCDController) syncMonitors() error {
    h := xxhash.New64()

    // - Get monitors from etcd ------------------------------------------------
    value, err := c.etcd.Get("monitors")
    if err != nil {
        logrus.Warnf("couldn't retrieve monitors from etcd, see: %v", err)
    } else {
        // - Get hash & compare to see if we need to do anything -------------------
        remoteMonitors := value

        reader := strings.NewReader(remoteMonitors)
        io.Copy(h, reader)
        hash := h.Sum64()

        if hash == c.lastMonitorHash {
            logrus.Debugf("skipping sync of monitors since hash is the same")
            return nil
        }
    }

    // - Fuck, upstream is different than we set it, update it again! ----------
    localMonitors, err := c.getWantedMonitorsAsString()
    if err != nil {
        return fmt.Errorf("couldn't form wanted monitors string, see: %v", err)
    }

    err = c.etcd.Set("monitors", localMonitors)
    if err != nil {
        return fmt.Errorf("couldn't update monitors in etcd, see: %v", err)
    }

    // - Rehash local hash -----------------------------------------------------
    h = xxhash.New64()
    reader := strings.NewReader(localMonitors)
    io.Copy(h, reader)
    c.lastMonitorHash = h.Sum64()

    return nil
}

// Stop stops the controller and blocks till everything is down
func (c *ETCDController) Stop() {
    if !c.running {
        return
    }

    go (func() { c.monitorsStopCh <- struct{}{} })()
    go (func() { c.statusStopCh <- struct{}{} })()

    for c.monitorLoopStarted || c.statusLoopStarted {
        time.Sleep(1 * time.Second)
    }

    c.running = false
}
