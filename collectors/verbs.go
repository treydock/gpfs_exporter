package collectors

import (
	"bytes"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type VerbsMetrics struct {
	Status string
}

type VerbsCollector struct {
	Status *prometheus.Desc
}

func init() {
	registerCollector("verbs", false, NewVerbsCollector)
}

func NewVerbsCollector() Collector {
	return &VerbsCollector{
		Status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "verbs", "status"),
			"GPFS verbs status, 1=started 0=not started", nil, nil),
	}
}

func (c *VerbsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Status
}

func (c *VerbsCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting verbs metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "verbs")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "verbs")
	}
}

func (c *VerbsCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	out, err := verbs()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "verbs")
		return err
	}
	metric, err := verbs_parse(out)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "verbs")
		return err
	}
	if metric.Status == "started" {
		ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, 1)
	} else {
		ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, 0)
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "verbs")
	return nil
}

func verbs() (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmfsadm", "test", "verbs", "status")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func verbs_parse(out string) (VerbsMetrics, error) {
	metric := VerbsMetrics{}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "VERBS") {
			continue
		}
		items := strings.Split(l, ": ")
		if len(items) == 2 {
			metric.Status = items[1]
			break
		}
	}
	return metric, nil
}
