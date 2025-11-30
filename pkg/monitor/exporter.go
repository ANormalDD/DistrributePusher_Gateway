package monitor

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	monitors   = map[string]*Monitor{}
	monitorsMu sync.RWMutex

	avgTimeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_monitor_avg_time_ms",
		Help: "Average processing time in milliseconds for monitor",
	}, []string{"monitor"})

	successRateGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_monitor_success_rate",
		Help: "Success rate (0..1) for monitor",
	}, []string{"monitor"})

	countGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_monitor_count",
		Help: "Number of samples in sliding window for monitor",
	}, []string{"monitor"})
)

func init() {
	// register metrics with default registry
	prometheus.MustRegister(avgTimeGauge)
	prometheus.MustRegister(successRateGauge)
	prometheus.MustRegister(countGauge)
}

// registerMonitor registers a monitor for metric collection.
func registerMonitor(m *Monitor) {
	if m == nil {
		return
	}
	monitorsMu.Lock()
	defer monitorsMu.Unlock()
	monitors[m.name] = m
}

// CollectMetrics samples all registered monitors and updates Prometheus gauges.
func CollectMetrics() {
	monitorsMu.RLock()
	defer monitorsMu.RUnlock()
	for name, m := range monitors {
		avg, succ, cnt := m.GetStats()
		// set metrics
		avgTimeGauge.WithLabelValues(name).Set(avg)
		successRateGauge.WithLabelValues(name).Set(succ)
		countGauge.WithLabelValues(name).Set(float64(cnt))
	}
}

// StartExporter starts a background sampler and an HTTP server that exposes /metrics.
// addr is the listen address, e.g. ":9100". interval is sampling interval.
func StartExporter(addr string, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// start sampler
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			CollectMetrics()
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	zap.L().Info(fmt.Sprintf("starting metrics exporter on %s", addr))
	return http.ListenAndServe(addr, mux)
}

// Handler returns an http.Handler that serves Prometheus metrics. Useful to mount into
// an existing HTTP server (for example Gin) as a route.
func Handler() http.Handler {
	return promhttp.Handler()
}
