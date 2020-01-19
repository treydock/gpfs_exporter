package collector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/treydock/gpfs_exporter/config"
)

var (
	availableScrapers = map[string]Scraper{
		"mmpmon": ScrapeMmpmon{},
		"mount":  ScrapeMount{},
	}
	scrapeDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

// Exporter collects GPFS metrics. It implements prometheus.Collector.
type Exporter struct {
	target       config.Target
	scrapers     []Scraper
	scrapeErrors *prometheus.CounterVec
	error        prometheus.Gauge
}

func New(target config.Target) *Exporter {
	var scrapers []Scraper
	for _, c := range target.Collectors {
		if scraper, ok := availableScrapers[c]; ok {
			scrapers = append(scrapers, scraper)
		} else {
			log.Errorf("Collector %s is not valid", c)
		}
	}

	return &Exporter{
		target:   target,
		scrapers: scrapers,
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping GPFS.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from GPFS resulted in an error (1 for error, 0 for success).",
		}),
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	e.scrape(ch)
	ch <- e.error
	e.scrapeErrors.Collect(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.target.Lock()
	defer e.target.Unlock()
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			label := scraper.Name()
			if err := scraper.Scrape(e.target, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.scrapeErrors.WithLabelValues(label).Inc()
				e.error.Set(1)
			}

		}(scraper)
	}
}
