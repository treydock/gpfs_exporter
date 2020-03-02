// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbsTimeout = kingpin.Flag("collector.verbs.timeout", "Timeout for collecting verbs information").Default("5").Int()
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*verbsTimeout)*time.Second)
	defer cancel()
	out, err := verbs(ctx)
	if ctx.Err() == context.DeadlineExceeded {
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "verbs")
		log.Error("Timeout executing verbs check")
		return nil
	}
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "verbs")
	if err != nil {
		return err
	}
	metric, err := verbs_parse(out)
	if err != nil {
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

func verbs(ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmfsadm", "test", "verbs", "status")
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
			metric.Status = strings.TrimSuffix(items[1], "\n")
			break
		}
	}
	return metric, nil
}
