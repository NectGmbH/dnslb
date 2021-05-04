package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics contains all logic for prometheus metrics
type Metrics struct {
	ErrorsTotal              prometheus.Counter
	ErrorsDNS                prometheus.Counter
	ErrorsETCD               prometheus.Counter
	LBUpdatesTotal           prometheus.Counter
	DNSZoneUpdates           *prometheus.CounterVec
	AgentsTotal              prometheus.Gauge
	AgentsUp                 prometheus.Gauge
	AgentsDown               prometheus.Gauge
	AgentsStatusAge          *prometheus.GaugeVec
	AgentsToHealthyEndpoints *prometheus.GaugeVec
	LBTotalEndpoints         *prometheus.GaugeVec
	LBHealthyEndpoints       *prometheus.GaugeVec
	LBUnhealthyEndpoints     *prometheus.GaugeVec
	DNSSyncTime              prometheus.Histogram
	ETCDSyncTime             prometheus.Histogram
	ETCDMonitorSyncTime      prometheus.Histogram
}

// Init initializes the metrics
func (m *Metrics) Init() error {
	// -- ErrorsTotal ----------------------------------------------------------
	m.ErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "general",
			Name:      "errors_total",
			Help:      "Total number of errors happened.",
		})

	err := prometheus.Register(m.ErrorsTotal)
	if err != nil {
		return fmt.Errorf("couldn't register ErrorsTotal counter, see: %v", err)
	}

	// -- ErrorsDNS ------------------------------------------------------------
	m.ErrorsDNS = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "dnscontroller",
			Name:      "errors_dns",
			Help:      "Total number of errors happened in the dns controller",
		})

	err = prometheus.Register(m.ErrorsDNS)
	if err != nil {
		return fmt.Errorf("couldn't register ErrorsDNS counter, see: %v", err)
	}

	// -- ErrorsETCD -----------------------------------------------------------
	m.ErrorsETCD = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "etcdcontroller",
			Name:      "errors_etcd",
			Help:      "Total number of errors happened in the etcd controller.",
		})

	err = prometheus.Register(m.ErrorsETCD)
	if err != nil {
		return fmt.Errorf("couldn't register ErrorsETCD counter, see: %v", err)
	}

	// -- LBUpdatesTotal -------------------------------------------------------
	m.LBUpdatesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "dnscontroller",
			Name:      "lb_updates_total",
			Help:      "Total number of lb updates passed to dnscontroller.",
		})

	err = prometheus.Register(m.LBUpdatesTotal)
	if err != nil {
		return fmt.Errorf("couldn't register LBUpdatesTotal counter, see: %v", err)
	}

	// -- DNSZoneUpdates -------------------------------------------------------
	m.DNSZoneUpdates = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "dnscontroller",
			Name:      "zone_updates_by_name",
			Help:      "Number of zone updates by name.",
		},
		[]string{"zone"},
	)

	err = prometheus.Register(m.DNSZoneUpdates)
	if err != nil {
		return fmt.Errorf("couldn't register DNSZoneUpdates counterVec, see: %v", err)
	}

	// -- AgentsTotal ----------------------------------------------------------
	m.AgentsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "agents_total",
			Help:      "Number of total agents registered in dnslb.",
		})

	err = prometheus.Register(m.AgentsTotal)
	if err != nil {
		return fmt.Errorf("couldn't register AgentsTotal gauge, see: %v", err)
	}

	// -- AgentsUp -------------------------------------------------------------
	m.AgentsUp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "agents_up",
			Help:      "Number of agents operating.",
		})

	err = prometheus.Register(m.AgentsUp)
	if err != nil {
		return fmt.Errorf("couldn't register AgentsUp gauge, see: %v", err)
	}

	// -- AgentsDown -----------------------------------------------------------
	m.AgentsDown = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "agents_down",
			Help:      "Number of agents with an unknown or down status.",
		})

	err = prometheus.Register(m.AgentsDown)
	if err != nil {
		return fmt.Errorf("couldn't register AgentsDown gauge, see: %v", err)
	}

	// -- AgentsStatusAge ------------------------------------------------------
	m.AgentsStatusAge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "agents_status_age",
			Help:      "Agents with the age of their most recent status",
		},
		[]string{"agent"})

	err = prometheus.Register(m.AgentsStatusAge)
	if err != nil {
		return fmt.Errorf("couldn't register AgentsStatusAge gauge, see: %v", err)
	}

	// -- AgentsToHealthyEndpoints ---------------------------------------------
	m.AgentsToHealthyEndpoints = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "agent_healthy_endpoints",
			Help:      "Agents with amount of healthy endpoints",
		},
		[]string{"agent"})

	err = prometheus.Register(m.AgentsToHealthyEndpoints)
	if err != nil {
		return fmt.Errorf("couldn't register AgentsToHealthyEndpoints gauge, see: %v", err)
	}

	// -- LBTotalEndpoints -----------------------------------------------------
	m.LBTotalEndpoints = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "lb_total_endpoints",
			Help:      "Loadbalancers with amount of configured endpoints",
		},
		[]string{"lb"})

	err = prometheus.Register(m.LBTotalEndpoints)
	if err != nil {
		return fmt.Errorf("couldn't register LBTotalEndpoints gauge, see: %v", err)
	}

	// -- LBHealthyEndpoints ---------------------------------------------------
	m.LBHealthyEndpoints = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "lb_healthy_endpoints",
			Help:      "Loadbalancers with amount of healthy endpoints",
		},
		[]string{"lb"})

	err = prometheus.Register(m.LBHealthyEndpoints)
	if err != nil {
		return fmt.Errorf("couldn't register LBHealthyEndpoints gauge, see: %v", err)
	}

	// -- LBUnhealthyEndpoints -------------------------------------------------
	m.LBUnhealthyEndpoints = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "etcdcontroller",
			Name:      "lb_unhealthy_endpoints",
			Help:      "Loadbalancers with amount of unhealthy or unknown endpoints",
		},
		[]string{"lb"})

	err = prometheus.Register(m.LBUnhealthyEndpoints)
	if err != nil {
		return fmt.Errorf("couldn't register LBUnhealthyEndpoints gauge, see: %v", err)
	}

	// -- DNSSyncTime ----------------------------------------------------------
	m.DNSSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "dnscontroller",
		Name:      "dns_sync_time",
		Help:      "Time the dns sync loop takes per cycle in ms.",
		Buckets:   prometheus.ExponentialBuckets(20, 2, 10),
	})

	err = prometheus.Register(m.DNSSyncTime)
	if err != nil {
		return fmt.Errorf("couldn't register DNSSyncTime gauge, see: %v", err)
	}

	// -- ETCDSyncTime ---------------------------------------------------------
	m.ETCDSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "etcdcontroller",
		Name:      "etcd_sync_time",
		Help:      "Time the etcd sync loop takes per cycle in ms.",
		Buckets:   prometheus.ExponentialBuckets(20, 2, 10),
	})

	err = prometheus.Register(m.ETCDSyncTime)
	if err != nil {
		return fmt.Errorf("couldn't register ETCDSyncTime gauge, see: %v", err)
	}

	// -- ETCDMonitorSyncTime --------------------------------------------------
	m.ETCDMonitorSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "etcdcontroller",
		Name:      "etcd_monitor_sync_time",
		Help:      "Time the etcd monitor sync loop takes per cycle in ms.",
		Buckets:   prometheus.ExponentialBuckets(20, 2, 10),
	})

	err = prometheus.Register(m.ETCDMonitorSyncTime)
	if err != nil {
		return fmt.Errorf("couldn't register ETCDMonitorSyncTime gauge, see: %v", err)
	}

	// -------------------------------------------------------------------------

	http.Handle("/metrics", promhttp.Handler())

	return nil
}
