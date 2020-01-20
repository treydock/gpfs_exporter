package main

import (
	"strings"

	"github.com/gofrs/flock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/treydock/gpfs_exporter/collectors"
	"github.com/treydock/gpfs_exporter/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	output      = kingpin.Flag("output", "Path to node exporter collected file").Required().String()
	filesystems = kingpin.Flag("filesystems", "Comma separated list of filesystems to query").Required().String()
	lockFile    = kingpin.Flag("lockfile", "Lock file path").Default("/tmp/gpfs_mmdf_exporter.lock").String()
)

/*
func GetMetrics(fs string) (collector.DFMetric, error) {
    out, err := collector.Mmdf(fs)
    if err != nil {
        return collector.DFMetric{}, err
    }
    dfMetric, err := collector.Parse_mmdf(out)
    if err != nil {
        return collector.DFMetric{}, err
    }
	return dfMetric, nil
}

type Collector struct {
    fs               string
	inodes_used      *prometheus.Desc
	inodes_free      *prometheus.Desc
	inodes_allocated *prometheus.Desc
	inodes_total     *prometheus.Desc
	fs_total         *prometheus.Desc
    fs_free         *prometheus.Desc
    fs_free_percent *prometheus.Desc
    metadata_total  *prometheus.Desc
    metadata_free   *prometheus.Desc
    metadata_free_percent  *prometheus.Desc
}

func NewCollector(fs string) *Collector {
	return &Collector{
        fs: fs,
		inodes_used:      collector.Inodes_used,
		inodes_free:      collector.Inodes_free,
		inodes_allocated: collector.Inodes_allocated,
		inodes_total:     collector.Inodes_total,
		fs_total:         collector.Fs_total,
		fs_free:         collector.Fs_free,
		fs_free_percent:         collector.Fs_free_percent,
        metadata_total: collector.Metadata_total,
        metadata_free: collector.Metadata_free,
        metadata_free_percent: collector.Metadata_free_percent,
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.inodes_used
	ch <- c.inodes_free
	ch <- c.inodes_allocated
	ch <- c.inodes_total
	ch <- c.fs_total
    ch <- c.fs_free
    ch <- c.fs_free_percent
    ch <- c.metadata_total
    ch <- c.metadata_free
    ch <- c.metadata_free_percent
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	metrics, err := GetMetrics(c.fs)
	if err != nil {
		log.Fatal(err)
	}
	ch <- prometheus.MustNewConstMetric(c.inodes_used, prometheus.GaugeValue, float64(metrics.InodesUsed), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_free, prometheus.GaugeValue, float64(metrics.InodesFree), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_allocated, prometheus.GaugeValue, float64(metrics.InodesAllocated), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_total, prometheus.GaugeValue, float64(metrics.InodesTotal), c.fs)
	ch <- prometheus.MustNewConstMetric(c.fs_total, prometheus.GaugeValue, float64(metrics.FSTotal), c.fs)
	ch <- prometheus.MustNewConstMetric(c.fs_free, prometheus.GaugeValue, float64(metrics.FSFree), c.fs)
	ch <- prometheus.MustNewConstMetric(c.fs_free_percent, prometheus.GaugeValue, float64(metrics.FSFreePercent), c.fs)
	ch <- prometheus.MustNewConstMetric(c.metadata_total, prometheus.GaugeValue, float64(metrics.MetadataTotal), c.fs)
	ch <- prometheus.MustNewConstMetric(c.metadata_free, prometheus.GaugeValue, float64(metrics.MetadataFree), c.fs)
	ch <- prometheus.MustNewConstMetric(c.metadata_free_percent, prometheus.GaugeValue, float64(metrics.MetadataFreePercent), c.fs)
}
*/

func collect() {
	mmdfFilesystems := strings.Split(*filesystems, ",")
	target := config.Target{MmdfFilesystems: mmdfFilesystems}
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewMmdfCollector(target))
	err := prometheus.WriteToTextfile(*output, registry)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	fileLock := flock.New(*lockFile)
	defer fileLock.Unlock()
	locked, err := fileLock.TryLock()
	if err != nil {
		log.Errorln("Unable to obtain lock on lock file", *lockFile)
		log.Fatal(err)
	}
	if !locked {
		log.Fatalf("Lock file %s is locked", *lockFile)
	}
	collect()
}
