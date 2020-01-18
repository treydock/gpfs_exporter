package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/treydock/gpfs_exporter/config"
)

const (
	// Exporter namespace.
	namespace = "gpfs"
)

// Scraper is minimal interface that let's you add new prometheus metrics to gpfs_exporter.
type Scraper interface {
	// Name of the Scraper. Should be unique.
	Name() string
	// Scrape collects data from GPFS
	Scrape(target config.Target, ch chan<- prometheus.Metric) error
}
