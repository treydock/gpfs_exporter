package collector

import (
	"os/exec"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/treydock/gpfs_exporter/config"
)

const (
	namespace = "gpfs"
)

var (
	execCommand = exec.Command
	availableCollectors = map[string]Collector{
		"mmpmon": MmpmonCollector{},
		"mount":  MountCollector{},
        "mmdf": MmdfCollector{},
	}
	collectDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

type Collector interface {
	Name() string
	Collect(target config.Target, ch chan<- prometheus.Metric) error
}

type Exporter struct {
	target       config.Target
	collectors     []Collector
	collectErrors *prometheus.CounterVec
	error        prometheus.Gauge
}

func New(target config.Target) *Exporter {
	var collectors []Collector
	for _, c := range target.Collectors {
		if collector, ok := availableCollectors[c]; ok {
			collectors = append(collectors, collector)
		} else {
			log.Errorf("Collector %s is not valid", c)
		}
	}

	return &Exporter{
		target:   target,
		collectors: collectors,
		collectErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
            Subsystem: "exporter",
			Name:      "collect_errors_total",
			Help:      "Total number of times an error occurred collecting GPFS.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
            Subsystem: "exporter",
			Name:      "last_collect_error",
			Help:      "Whether the last collect of metrics from GPFS resulted in an error (1 for error, 0 for success).",
		}),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
    e.collectErrors.Describe(ch)
    e.error.Describe(ch)
    ch <- collectDuration
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.collect(ch)
	ch <- e.error
	e.collectErrors.Collect(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) {
	e.target.Lock()
	defer e.target.Unlock()
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, collector := range e.collectors {
		wg.Add(1)
		go func(collector Collector) {
			defer wg.Done()
			label := collector.Name()
			if err := collector.Collect(e.target, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.collectErrors.WithLabelValues(label).Inc()
				e.error.Set(1)
			}

		}(collector)
	}
}
